#!/usr/bin/env python
"""RunPod serverless handler: parse every page under a prefix with PaddleOCR-VL-1.6.

Reads images/PDFs from a bucket, runs the VL doc-parsing pipeline (layout local +
VLM on an in-container vLLM server), and writes one markdown file per page back.
"""
from __future__ import annotations

import logging
import os
import tempfile
import time
from pathlib import Path

import boto3
import runpod
from botocore.config import Config

import vlocr

logging.basicConfig(
    level=os.environ.get("LOG_LEVEL", "INFO").upper(),
    format="%(asctime)s %(levelname)s %(name)s %(message)s",
)
log = logging.getLogger("handler")

# "vl" (VL only) | "both" (VL + v6 line boxes) | "v6" (PP-OCRv6 only).
# Per-job "mode" overrides this.
# Engines a job may ask for. More may be added; a job lists the ones it wants.
KNOWN_MODES = ("v6", "vl")
_client = None
_server_proc = None
_v6 = None


def v6_ocr():
    """PP-OCRv6 det+rec. Seconds to load, no GPU server involved."""
    global _v6
    if _v6 is None:
        import ocr_v6
        t0 = time.perf_counter()
        _v6 = ocr_v6.build_ocr(os.environ.get("VL_DEVICE", "gpu:0"))
        log.info("PP-OCRv6 ready in %.1fs", time.perf_counter() - t0)
    return _v6


def client():
    """Start the vLLM server and load the paddle models once per worker process.

    Called lazily: a v6-only job never reaches this, so it never pays the vLLM
    weight load + CUDA-graph capture (~2 min).
    """
    global _client, _server_proc
    if _client is None:
        t0 = time.perf_counter()
        port = int(os.environ.get("VL_SERVER_PORT", "8080"))
        url = os.environ.get("VL_SERVER_URL")  # set to reuse an external server
        if not url:
            _server_proc = vlocr.start_vllm_server(port=port, log=None)
            vlocr.wait_ready(port=port, proc=_server_proc)
            url = f"http://127.0.0.1:{port}/v1"
        else:
            vlocr.wait_ready(port=int(url.split(":")[2].split("/")[0]))
        _client = vlocr.build_client(url, device=os.environ.get("VL_DEVICE", "gpu:0"))
        log.info("VL ready in %.1fs", time.perf_counter() - t0)
    return _client


def make_s3(inp):
    return boto3.client(
        "s3",
        aws_access_key_id=inp["aws_access_key_id"],
        aws_secret_access_key=inp["aws_secret_access_key"],
        region_name=inp.get("region", "us-east-1"),
        endpoint_url=inp.get("endpoint_url") or None,
        config=Config(retries={"max_attempts": 5, "mode": "standard"},
                      s3={"addressing_style": "path"}),
    )


def list_inputs(s3, bucket, prefix):
    keys = []
    for page in s3.get_paginator("list_objects_v2").paginate(Bucket=bucket, Prefix=prefix):
        for obj in page.get("Contents", []):
            if not obj["Key"].endswith("/"):
                keys.append(obj["Key"])
    return sorted(keys)


def handler(job):
    inp = job["input"]
    bucket = inp["bucket"]
    input_prefix = inp["input_prefix"]
    output_prefix = inp["output_prefix"]
    modes = inp.get("modes")
    if not isinstance(modes, (list, tuple)) or not modes:
        return {"error": "'modes' is required and must be a non-empty array, "
                         f"e.g. [\"v6\"] or [\"v6\", \"vl\"]; known: {list(KNOWN_MODES)}"}
    modes = [str(m).lower() for m in modes]
    unknown = [m for m in modes if m not in KNOWN_MODES]
    if unknown:
        return {"error": f"unknown modes {unknown}; known: {list(KNOWN_MODES)}"}
    want_v6, want_vl = "v6" in modes, "vl" in modes

    try:
        vl_sampling = vlocr.sampling_args(inp.get("temperature"), inp.get("top_p"))
    except (TypeError, ValueError):
        return {"error": "'temperature' and 'top_p' must be numbers"}

    started = time.perf_counter()
    log.info("job started: modes=%s sampling=%s bucket=%s input=%s output=%s",
             modes, vl_sampling or "pipeline default", bucket, input_prefix, output_prefix)

    s3 = make_s3(inp)
    keys = list_inputs(s3, bucket, input_prefix)
    log.info("found %d object(s) under %s", len(keys), input_prefix)
    if not keys:
        return {"pages": [], "failed": []}

    with tempfile.TemporaryDirectory() as tmp:
        in_dir = Path(tmp) / "in"
        out_dir = Path(tmp) / "out"
        in_dir.mkdir()

        key_of, paths, downloaded = {}, [], 0
        for key in keys:
            local = in_dir / Path(key).name
            s3.download_file(bucket, key, str(local))
            downloaded += local.stat().st_size
            key_of[str(local)] = key
            paths.append(local)
        log.info("downloaded %d page(s), %.1f MB", len(paths), downloaded / (1 << 20))

        done = 0

        def progress(_r):
            nonlocal done
            done += 1
            runpod.serverless.progress_update(job, f"{done}/{len(paths)} pages")

        t0 = time.perf_counter()
        # each requested engine runs independently and writes its own files
        results = []
        if want_v6:
            results += vlocr.parse_files_v6(v6_ocr(), paths, out_dir, base=in_dir,
                                            on_result=progress)
        if want_vl:
            results += vlocr.parse_files(client(), paths, out_dir, base=in_dir,
                                         on_result=progress, predict_args=vl_sampling)
        log.info("parsed %d page(s) in %.1fs", len(results), time.perf_counter() - t0)

        pages, failed, uploaded = [], [], 0
        for r in results:
            key = key_of.get(r["source"], r["source"])
            if r.get("error"):
                log.warning("page %s failed: %s", key, r["error"])
                failed.append({"input_key": key, "error": r["error"]})
                continue
            keys_out, ok = [], True
            for local in r.get("outputs") or []:
                out_key = output_prefix + Path(local).name
                try:
                    s3.upload_file(local, bucket, out_key)
                    uploaded += Path(local).stat().st_size
                    keys_out.append(out_key)
                except Exception as exc:
                    log.error("upload of %s failed: %s", out_key, exc)
                    failed.append({"input_key": key, "error": f"upload failed: {exc}"})
                    ok = False
                    break
            if not ok:
                continue
            entry = {"input_key": key, "output_keys": keys_out,
                     "seconds": round(r.get("seconds", 0), 2)}
            for f in ("chars", "blocks", "lines"):
                if r.get(f) is not None:
                    entry[f] = r[f]
            pages.append(entry)
        log.info("uploaded %d result(s), %.1f MB", len(pages), uploaded / (1 << 20))

    log.info("job done: %d page(s), %d failed, %.1fs",
             len(pages), len(failed), time.perf_counter() - started)
    return {"pages": pages, "failed": failed}


if __name__ == "__main__":
    # v6 loads in seconds, so warm it always. The vLLM server costs ~2 min and is
    # started lazily, by the first job that actually needs it.
    v6_ocr()
    runpod.serverless.start({"handler": handler})
