#!/usr/bin/env python
from __future__ import annotations

import argparse
import json
import os
import sys
import time
from pathlib import Path

import cv2
import numpy as np

CONFIG = os.environ.get("DOCPP_CONFIG", "/opt/doc_preprocessor.yaml")
DEVICE = os.environ.get("DOCPP_DEVICE", "gpu:0")
IMAGE_EXT = {".jpg", ".jpeg", ".png", ".bmp", ".tif", ".tiff", ".webp"}
PDF_EXT = {".pdf"}


def parse_args(argv=None):
    p = argparse.ArgumentParser(
        prog="docpp",
        description="Document image preprocessing (orientation + unwarping) on GPU.",
    )
    p.add_argument("-i", "--input", default="/data/input",
                   help="Directory of images (or a single file). Default: /data/input")
    p.add_argument("-o", "--output", default="/data/output",
                   help="Output directory. Default: /data/output")
    p.add_argument("-r", "--recursive", action="store_true",
                   help="Recurse into subdirectories, mirroring the tree in the output.")
    p.add_argument("--device", default=DEVICE,
                   help="Inference device, e.g. gpu:0 or cpu. Default: gpu:0")
    p.add_argument("-b", "--batch-size", type=int, default=4,
                   help="Images per forward pass. Default: 4")
    p.add_argument("--no-orientation", dest="orientation", action="store_false",
                   help="Skip the 0/90/180/270 orientation classifier.")
    p.add_argument("--no-unwarp", dest="unwarp", action="store_false",
                   help="Skip UVDoc geometric unwarping.")
    p.add_argument("--jpeg-quality", type=int, default=95,
                   help="Quality for .jpg output. Default: 95")
    p.add_argument("--png", action="store_true", help="Write PNG instead of JPEG.")
    p.add_argument("--json", dest="save_json", action="store_true",
                   help="Also write the per-image result JSON (detected angle etc.).")
    p.add_argument("--no-warmup", dest="warmup", action="store_false",
                   help="Skip the synthetic warm-up pass at startup.")
    p.add_argument("-q", "--quiet", action="store_true", help="Only print the summary.")
    return p.parse_args(argv)


def collect(input_path: Path, recursive: bool):
    if input_path.is_file():
        return [input_path]
    globber = input_path.rglob if recursive else input_path.glob
    exts = IMAGE_EXT | PDF_EXT
    return sorted(p for p in globber("*") if p.is_file() and p.suffix.lower() in exts)


def load(path: Path):
    data = np.fromfile(str(path), dtype=np.uint8)
    img = cv2.imdecode(data, cv2.IMREAD_COLOR)
    if img is None:
        raise ValueError(f"could not decode {path}")
    return img


def batched(seq, n):
    for i in range(0, len(seq), n):
        yield seq[i:i + n]


def build_pipeline(device: str = DEVICE, config: str = CONFIG, warmup: bool = True,
                   batch_size: int = 2, orientation: bool = True, unwarp: bool = True):
    """Load the models once. A serverless worker reuses the result for every job."""
    from paddlex import create_pipeline

    pipeline = create_pipeline(pipeline=config, device=device)

    if warmup:
        dummy = np.full((1024, 768, 3), 255, dtype=np.uint8)
        list(pipeline.predict(
            input=[dummy] * max(1, min(batch_size, 2)),
            use_doc_orientation_classify=orientation,
            use_doc_unwarping=unwarp,
        ))

    return pipeline


def dewarp_files(pipeline, files, out_dir, base=None, recursive=False, batch_size=4,
                 orientation=True, unwarp=True, png=False, jpeg_quality=95,
                 save_json=False, on_result=None):
    """Dewarp every file into out_dir.

    Returns one dict per result. A file that cannot be read yields a dict with
    an "error" key and no output, so one bad page never stops the rest.
    """
    out_root = Path(out_dir)
    out_root.mkdir(parents=True, exist_ok=True)

    files = [Path(f) for f in files]
    if base is None:
        base = files[0].parent if files else out_root
    base = Path(base)

    ext = ".png" if png else ".jpg"
    write_params = [] if png else [cv2.IMWRITE_JPEG_QUALITY, jpeg_quality]
    results = []

    for chunk in batched(files, max(1, batch_size)):
        images, kept = [], []
        for path in chunk:
            if path.suffix.lower() in PDF_EXT:
                kept.append(path)
                images.append(str(path))
                continue
            try:
                images.append(load(path))
                kept.append(path)
            except Exception as exc:
                results.append({"source": str(path), "output": None, "error": str(exc)})
        if not images:
            continue

        t0 = time.perf_counter()
        predicted = list(pipeline.predict(
            input=images,
            use_doc_orientation_classify=orientation,
            use_doc_unwarping=unwarp,
        ))
        elapsed = (time.perf_counter() - t0) / max(len(predicted), 1)

        idx = -1
        prev_key = object()
        for res in predicted:
            key = res.get("input_path")
            page = res.get("page_index")
            if key != prev_key or page in (None, 0):
                idx += 1
                prev_key = key
            path = kept[min(idx, len(kept) - 1)]

            rel = path.relative_to(base) if recursive else Path(path.name)
            stem = rel.with_suffix("")
            if page is not None:
                stem = stem.with_name(f"{stem.name}_p{page:04d}")
            dst = out_root / stem.with_suffix(ext)
            dst.parent.mkdir(parents=True, exist_ok=True)

            img = np.asarray(res["output_img"])
            cv2.imwrite(str(dst), img, write_params)

            result = {
                "source": str(path),
                "output": str(dst),
                "error": None,
                "angle": res.get("angle"),
                "page_index": page,
                "width": int(img.shape[1]),
                "height": int(img.shape[0]),
                "seconds": elapsed,
            }
            if save_json:
                dst.with_suffix(".json").write_text(json.dumps(result, indent=2))

            results.append(result)
            if on_result is not None:
                on_result(result)

    return results


def main(argv=None) -> int:
    args = parse_args(argv)
    src_root = Path(args.input)
    if not src_root.exists():
        print(f"error: input path {src_root} does not exist", file=sys.stderr)
        return 2
    out_root = Path(args.output)
    out_root.mkdir(parents=True, exist_ok=True)

    files = collect(src_root, args.recursive)
    if not files:
        print(f"error: no images found under {src_root}", file=sys.stderr)
        return 2

    t_load = time.perf_counter()
    pipeline = build_pipeline(
        device=args.device,
        warmup=args.warmup,
        batch_size=args.batch_size,
        orientation=args.orientation,
        unwarp=args.unwarp,
    )
    t_load = time.perf_counter() - t_load

    def report(result):
        if not args.quiet:
            print(f"{result['source']} -> {result['output']}  angle={result.get('angle')} "
                  f"{result['width']}x{result['height']}  ({result['seconds']:.3f}s)")

    t_all = time.perf_counter()
    results = dewarp_files(
        pipeline,
        files,
        out_root,
        base=src_root if src_root.is_dir() else src_root.parent,
        recursive=args.recursive,
        batch_size=args.batch_size,
        orientation=args.orientation,
        unwarp=args.unwarp,
        png=args.png,
        jpeg_quality=args.jpeg_quality,
        save_json=args.save_json,
        on_result=report,
    )
    t_all = time.perf_counter() - t_all

    ok = sum(1 for r in results if r.get("error") is None)
    failed = len(results) - ok
    for result in results:
        if result.get("error"):
            print(f"skip {result['source']}: {result['error']}", file=sys.stderr)

    print(f"\nmodel load and warm-up {t_load:.2f}s")
    print(f"processed {ok} image(s) in {t_all:.2f}s "
          f"({t_all / max(ok, 1):.3f}s each), {failed} failed")
    return 1 if failed and not ok else 0


if __name__ == "__main__":
    raise SystemExit(main())
