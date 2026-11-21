#!/usr/bin/env python
"""PaddleOCR-VL-1.6 document parsing.

Two GPU processes cooperate:
  * a vLLM server (started here as a subprocess) serves the 0.9B VLM;
  * this process runs the paddle layout/orientation/unwarp models and calls the
    server over its OpenAI-compatible API.

Keeping them in separate processes lets the vLLM (torch) stack and the paddle
stack coexist in one image without sharing an interpreter.

Each engine writes everything it produces; nothing is fused or post-processed.

CLI, for local testing without S3/RunPod:
    python vlocr.py -i /path/page.png -o /path/out            # spawns a server
    python vlocr.py -i page.png -o out --server-url http://127.0.0.1:8080/v1
"""
from __future__ import annotations

import argparse
import json
import os
import subprocess
import sys
import time
import urllib.request
from pathlib import Path

MODEL_NAME = os.environ.get("VL_MODEL_NAME", "PaddleOCR-VL-1.6-0.9B")
PIPELINE_VERSION = os.environ.get("VL_PIPELINE_VERSION", "v1.6")
DEVICE = os.environ.get("VL_DEVICE", "gpu:0")
SERVER_PORT = int(os.environ.get("VL_SERVER_PORT", "8080"))
# absolute path so the base-env `paddleocr` (with genai_server) is used, not a venv shadow
PADDLEOCR_BIN = os.environ.get("PADDLEOCR_BIN", "/usr/local/bin/paddleocr")
IMAGE_EXT = {".jpg", ".jpeg", ".png", ".bmp", ".tif", ".tiff", ".webp", ".pdf"}


def start_vllm_server(port: int = SERVER_PORT, model_name: str = MODEL_NAME,
                      gpu_mem_util: float | None = None, log=None):
    """Launch `paddleocr genai_server` (vLLM backend). Returns the Popen handle."""
    cmd = [PADDLEOCR_BIN, "genai_server", "--model_name", model_name,
           "--host", "127.0.0.1", "--port", str(port), "--backend", "vllm"]
    if gpu_mem_util is None:
        gpu_mem_util = os.environ.get("VL_GPU_MEM_UTIL")
    if gpu_mem_util:
        cmd += ["--gpu-memory-utilization", str(gpu_mem_util)]
    env = dict(os.environ)  # inherits PADDLE_PDX_CACHE_HOME + offline flags
    out = log or sys.stderr
    print(f"[vlocr] starting vLLM server: {' '.join(cmd)}", file=out, flush=True)
    return subprocess.Popen(cmd, stdout=out, stderr=out, env=env)


def wait_ready(port: int = SERVER_PORT, timeout: float = 600.0, proc=None) -> None:
    url = f"http://127.0.0.1:{port}/v1/models"
    deadline = time.time() + timeout
    while time.time() < deadline:
        if proc is not None and proc.poll() is not None:
            raise RuntimeError(f"vLLM server exited early (code {proc.returncode})")
        try:
            with urllib.request.urlopen(url, timeout=5) as r:
                if r.status == 200 and b"PaddleOCR-VL" in r.read():
                    return
        except Exception:
            pass
        time.sleep(3)
    raise TimeoutError(f"vLLM server not ready after {timeout}s")


def build_client(server_url: str, device: str = DEVICE):
    """The layout/orientation/unwarp models load here; VL recognition is remote."""
    from paddleocr import PaddleOCRVL
    return PaddleOCRVL(
        pipeline_version=PIPELINE_VERSION,
        vl_rec_backend="vllm-server",
        vl_rec_server_url=server_url,
        device=device,
    )


def sampling_args(temperature=None, top_p=None) -> dict:
    """The sampling the VL model runs with, for `predict`.

    A value left out keeps what the pipeline picks, which is temperature 1 with
    top_p 0. That top_p is outside the range the server accepts, so what it does
    with it belongs to the server version, not to this code. Naming both leaves
    nothing to the version.
    """
    args = {}
    if temperature is not None:
        args["temperature"] = float(temperature)
    if top_p is not None:
        args["top_p"] = float(top_p)
    return args


def _raw(res):
    """The pipeline result verbatim: layout boxes with scores, parsing blocks,
    model settings, page size -- whatever this version produces."""
    j = res.json() if callable(getattr(res, "json", None)) else getattr(res, "json", None)
    if isinstance(j, dict) and "res" in j:
        j = j["res"]
    return j


def _markdown_of(res) -> str:
    md = getattr(res, "markdown", None)
    if isinstance(md, dict):
        md = md.get("markdown_texts") or md.get("markdown") or md.get("text")
    if isinstance(md, str) and md.strip():
        return md
    try:
        return res.get("markdown") or res.get("text") or ""
    except Exception:
        return str(res)


def collect(input_path: Path):
    if input_path.is_file():
        return [input_path]
    return sorted(p for p in input_path.rglob("*")
                  if p.is_file() and p.suffix.lower() in IMAGE_EXT)


def parse_files(client, files, out_dir, base=None, on_result=None, predict_args=None):
    """PaddleOCR-VL: <name>.md (reading-order Markdown) and <name>.json (raw result)."""
    out_root = Path(out_dir)
    out_root.mkdir(parents=True, exist_ok=True)
    files = [Path(f) for f in files]
    base = Path(base) if base else (files[0].parent if files else out_root)
    predict_args = predict_args or {}
    results = []
    for path in files:
        t0 = time.perf_counter()
        try:
            # The pipeline merges paragraphs across column breaks (its default), so a
            # block can come back with no text -- it sits in its neighbour, not lost.
            pages = list(client.predict(str(path), **predict_args))
            text = "\n\n".join(p for p in (_markdown_of(r) for r in pages) if p)
            raw = [_raw(r) for r in pages]
            doc = raw[0] if len(raw) == 1 else {"pages": raw}

            rel = path.relative_to(base) if base in path.parents else Path(path.name)
            dst = out_root / rel.with_suffix(".md")
            dst.parent.mkdir(parents=True, exist_ok=True)
            dst.write_text(text, encoding="utf-8")
            jdst = dst.with_suffix(".json")
            jdst.write_text(json.dumps(doc, ensure_ascii=False, indent=2, default=str),
                            encoding="utf-8")
            blocks = sum(len((p or {}).get("parsing_res_list") or []) for p in raw)
            result = {"source": str(path), "error": None,
                      "outputs": [str(dst), str(jdst)],
                      "chars": len(text), "blocks": blocks,
                      "seconds": time.perf_counter() - t0}
        except Exception as exc:
            result = {"source": str(path), "error": str(exc), "outputs": [],
                      "seconds": time.perf_counter() - t0}
        results.append(result)
        if on_result is not None:
            on_result(result)
    return results


def parse_files_v6(ocr, files, out_dir, base=None, on_result=None):
    """PP-OCRv6: <name>.v6.json (raw result + flattened lines) and <name>.v6.txt.

    Distinct names so a job asking for both engines keeps both sets of files.
    Its dictionary has no polytonic Greek (accented vowels are dropped) and there
    is no cross-column reading order.
    """
    import ocr_v6 as _v6

    out_root = Path(out_dir)
    out_root.mkdir(parents=True, exist_ok=True)
    files = [Path(f) for f in files]
    base = Path(base) if base else (files[0].parent if files else out_root)
    results = []
    for path in files:
        t0 = time.perf_counter()
        try:
            raw, lines, text = _v6.predict(ocr, str(path))
            rel = path.relative_to(base) if base in path.parents else Path(path.name)
            stem = (out_root / rel).with_suffix("")
            stem.parent.mkdir(parents=True, exist_ok=True)
            jdst = stem.with_name(stem.name + ".v6.json")
            tdst = stem.with_name(stem.name + ".v6.txt")
            jdst.write_text(json.dumps({"raw": raw, "lines": lines},
                                       ensure_ascii=False, indent=2, default=str),
                            encoding="utf-8")
            tdst.write_text(text, encoding="utf-8")
            result = {"source": str(path), "error": None,
                      "outputs": [str(jdst), str(tdst)],
                      "chars": len(text), "lines": len(lines),
                      "seconds": time.perf_counter() - t0}
        except Exception as exc:
            result = {"source": str(path), "error": str(exc), "outputs": [],
                      "seconds": time.perf_counter() - t0}
        results.append(result)
        if on_result is not None:
            on_result(result)
    return results


def main(argv=None) -> int:
    ap = argparse.ArgumentParser(prog="vlocr", description="PaddleOCR-VL-1.6 / PP-OCRv6.")
    ap.add_argument("-i", "--input", required=True, help="Image/PDF file or a directory.")
    ap.add_argument("-o", "--output", default="out", help="Output directory. Default: out")
    ap.add_argument("--modes", nargs="+", required=True, choices=("v6", "vl"),
                    help="Engines to run, e.g. --modes v6  |  --modes v6 vl")
    ap.add_argument("--server-url", default=None,
                    help="Existing vLLM server /v1 URL. If omitted, one is spawned.")
    ap.add_argument("--device", default=DEVICE, help="Client device. Default: gpu:0")
    ap.add_argument("--port", type=int, default=SERVER_PORT)
    ap.add_argument("--temperature", type=float, default=None,
                    help="VL sampling temperature. Default: the pipeline's own.")
    ap.add_argument("--top-p", type=float, default=None,
                    help="VL sampling top_p. Default: the pipeline's own.")
    args = ap.parse_args(argv)

    src = Path(args.input)
    files = collect(src)
    if not files:
        print(f"error: no images under {src}", file=sys.stderr)
        return 2
    base = src if src.is_dir() else src.parent
    want_v6, want_vl = "v6" in args.modes, "vl" in args.modes

    def report(r):
        if r["error"]:
            print(f"skip {r['source']}: {r['error']}", file=sys.stderr)
        else:
            print(f"{r['source']} -> {', '.join(Path(o).name for o in r['outputs'])} "
                  f"({r['seconds']:.1f}s)")

    results = []
    if want_v6:
        import ocr_v6 as _v6
        t0 = time.perf_counter()
        ocr = _v6.build_ocr(args.device)
        print(f"[vlocr] v6 ready in {time.perf_counter()-t0:.1f}s", file=sys.stderr)
        results += parse_files_v6(ocr, files, args.output, base=base, on_result=report)

    if want_vl:
        proc = None
        server_url = args.server_url
        if not server_url:
            proc = start_vllm_server(port=args.port)
            wait_ready(port=args.port, proc=proc)
            server_url = f"http://127.0.0.1:{args.port}/v1"
        else:
            wait_ready(port=int(server_url.split(":")[2].split("/")[0]))
        try:
            client = build_client(server_url, device=args.device)
            results += parse_files(client, files, args.output, base=base, on_result=report,
                                   predict_args=sampling_args(args.temperature, args.top_p))
        finally:
            if proc is not None:
                proc.terminate()

    ok = sum(1 for r in results if not r["error"])
    print(f"\n{ok}/{len(results)} result(s) written")
    return 0 if ok else 1


if __name__ == "__main__":
    raise SystemExit(main())
