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
ORI_MODEL_DIR = "/opt/paddlex/official_models/PP-LCNet_x1_0_doc_ori"
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
    p.add_argument("--orientation", choices=("tiles", "whole", "off"), default="tiles",
                   help="How to decide page rotation. 'tiles' votes the classifier "
                        "over a 3x3 grid of crops (far more accurate on photos); "
                        "'whole' is PaddleX's stock single pass over the full frame; "
                        "'off' never rotates. Default: tiles")
    p.add_argument("--orientation-grid", type=int, default=3,
                   help="Grid size for --orientation tiles. Raise it for books with "
                        "small type: each tile's short side is downscaled to 256px "
                        "before classification, so grid N gives a "
                        "(height/N)/256 reduction. Default: 3")
    p.add_argument("--orientation-keep", type=float, default=0.5,
                   help="Fraction of tiles to actually classify, the inkiest first. "
                        "Blank margins vote randomly and dilute the result, so "
                        "dropping them raises confidence and cuts cost. Default: 0.5")
    p.add_argument("--no-orientation", dest="orientation", action="store_const", const="off",
                   help="Alias for --orientation off.")
    p.add_argument("--exif", choices=("apply", "ignore"), default="apply",
                   help="Honour the JPEG EXIF Orientation tag, or ignore it and use "
                        "the stored pixels as-is. Default: apply")
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


def load(path: Path, exif: str = "apply"):
    data = np.fromfile(str(path), dtype=np.uint8)
    flags = cv2.IMREAD_COLOR
    if exif == "ignore":
        flags |= cv2.IMREAD_IGNORE_ORIENTATION
    img = cv2.imdecode(data, flags)
    if img is None:
        raise ValueError(f"could not decode {path}")
    return img


CORRECTIONS = {90: cv2.ROTATE_90_COUNTERCLOCKWISE, 180: cv2.ROTATE_180,
               270: cv2.ROTATE_90_CLOCKWISE}


def texture(tile):
    small = cv2.cvtColor(cv2.resize(tile, (128, 128)), cv2.COLOR_BGR2GRAY)
    return float(cv2.Laplacian(small, cv2.CV_32F).var())


def tile_orientation(model, img, grid: int, keep: float = 0.5):
    h, w = img.shape[:2]
    crops = [img[r * h // grid:(r + 1) * h // grid, c * w // grid:(c + 1) * w // grid]
             for r in range(grid) for c in range(grid)]
    if keep < 1.0 and len(crops) > 2:
        crops.sort(key=texture, reverse=True)
        crops = crops[:max(2, round(len(crops) * keep))]
    weights = {}
    for res in model.predict(crops):
        i = int(np.argmax(res["scores"]))
        angle = int(res["label_names"][i])
        weights[angle] = weights.get(angle, 0.0) + float(res["scores"][i])
    angle = max(weights, key=weights.get)
    return angle, weights[angle] / sum(weights.values())


def batched(seq, n):
    for i in range(0, len(seq), n):
        yield seq[i:i + n]


class Preprocessor:
    """The loaded models plus the settings that decide how they run."""

    def __init__(self, pipeline, ori_model, orientation, unwarp, grid, keep):
        self.pipeline = pipeline
        self.ori_model = ori_model
        self.orientation = orientation
        self.unwarp = unwarp
        self.grid = grid
        self.keep = keep

    def predict(self, images):
        return self.pipeline.predict(
            input=images,
            use_doc_orientation_classify=(self.orientation == "whole"),
            use_doc_unwarping=self.unwarp,
        )

    def rotate(self, img):
        """Return the upright image and the angle removed, or (img, None)."""
        if self.ori_model is None:
            return img, None
        angle, _ = tile_orientation(self.ori_model, img, self.grid, self.keep)
        return (cv2.rotate(img, CORRECTIONS[angle]) if angle else img), angle


def build_pipeline(device: str = DEVICE, config: str = CONFIG, warmup: bool = True,
                   batch_size: int = 2, orientation: str = "tiles", unwarp: bool = True,
                   grid: int = 3, keep: float = 0.5):
    """Load the models once. A serverless worker reuses the result for every job."""
    from paddlex import create_model, create_pipeline

    pipeline = create_pipeline(pipeline=config, device=device)
    ori_model = None
    if orientation == "tiles":
        ori_model = create_model("PP-LCNet_x1_0_doc_ori", model_dir=ORI_MODEL_DIR,
                                 device=device)

    preprocessor = Preprocessor(pipeline, ori_model, orientation, unwarp, grid, keep)

    if warmup:
        dummy = np.full((1024, 768, 3), 255, dtype=np.uint8)
        list(preprocessor.predict([dummy] * max(1, min(batch_size, 2))))
        preprocessor.rotate(dummy)

    return preprocessor


def dewarp_files(preprocessor, files, out_dir, base=None, recursive=False, batch_size=4,
                 exif="apply", png=False, jpeg_quality=95,
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
                kept.append((path, None))
                images.append(str(path))
                continue
            try:
                img, angle = preprocessor.rotate(load(path, exif))
                images.append(img)
                kept.append((path, angle))
            except Exception as exc:
                results.append({"source": str(path), "output": None, "error": str(exc)})
        if not images:
            continue

        t0 = time.perf_counter()
        predicted = list(preprocessor.predict(images))
        elapsed = (time.perf_counter() - t0) / max(len(predicted), 1)

        idx = -1
        prev_key = object()
        for res in predicted:
            key = res.get("input_path")
            page = res.get("page_index")
            if key != prev_key or page in (None, 0):
                idx += 1
                prev_key = key
            path, own_angle = kept[min(idx, len(kept) - 1)]

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
                "angle": res.get("angle") if own_angle is None else own_angle,
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
    preprocessor = build_pipeline(
        device=args.device,
        warmup=args.warmup,
        batch_size=args.batch_size,
        orientation=args.orientation,
        unwarp=args.unwarp,
        grid=args.orientation_grid,
        keep=args.orientation_keep,
    )
    t_load = time.perf_counter() - t_load

    def report(result):
        if not args.quiet:
            print(f"{result['source']} -> {result['output']}  angle={result.get('angle')} "
                  f"{result['width']}x{result['height']}  ({result['seconds']:.3f}s)")

    t_all = time.perf_counter()
    results = dewarp_files(
        preprocessor,
        files,
        out_root,
        base=src_root if src_root.is_dir() else src_root.parent,
        recursive=args.recursive,
        batch_size=args.batch_size,
        exif=args.exif,
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
