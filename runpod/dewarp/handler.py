#!/usr/bin/env python
"""RunPod serverless handler: dewarp every object under a prefix."""
from __future__ import annotations

import os
import tempfile
from pathlib import Path

import boto3
import runpod
from botocore.config import Config

import docpp

_pipeline = None


def pipeline():
    """The models load once per worker process, not once per job."""
    global _pipeline
    if _pipeline is None:
        _pipeline = docpp.build_pipeline(device=os.environ.get("DOCPP_DEVICE", docpp.DEVICE))
    return _pipeline


def make_s3(inp):
    return boto3.client(
        "s3",
        aws_access_key_id=inp["aws_access_key_id"],
        aws_secret_access_key=inp["aws_secret_access_key"],
        region_name=inp.get("region", "us-east-1"),
        endpoint_url=inp.get("endpoint_url") or None,
        config=Config(
            retries={"max_attempts": 5, "mode": "standard"},
            s3={"addressing_style": "path"},
        ),
    )


def list_inputs(s3, bucket, prefix):
    keys = []
    paginator = s3.get_paginator("list_objects_v2")
    for page in paginator.paginate(Bucket=bucket, Prefix=prefix):
        for obj in page.get("Contents", []):
            key = obj["Key"]
            if not key.endswith("/"):
                keys.append(key)
    return sorted(keys)


def handler(job):
    inp = job["input"]
    bucket = inp["bucket"]
    input_prefix = inp["input_prefix"]
    output_prefix = inp["output_prefix"]
    quality = int(inp.get("jpeg_quality", 95))

    s3 = make_s3(inp)
    keys = list_inputs(s3, bucket, input_prefix)
    if not keys:
        return {"pages": [], "failed": []}

    with tempfile.TemporaryDirectory() as tmp:
        in_dir = Path(tmp) / "in"
        out_dir = Path(tmp) / "out"
        in_dir.mkdir()

        key_of = {}
        paths = []
        for key in keys:
            local = in_dir / Path(key).name
            s3.download_file(bucket, key, str(local))
            key_of[str(local)] = key
            paths.append(local)

        done = 0

        def progress(_result):
            nonlocal done
            done += 1
            runpod.serverless.progress_update(job, f"{done}/{len(paths)} pages")

        results = docpp.dewarp_files(
            pipeline(),
            paths,
            out_dir,
            base=in_dir,
            jpeg_quality=quality,
            on_result=progress,
        )

        pages, failed = [], []
        for result in results:
            key = key_of.get(result["source"], result["source"])

            if result.get("error"):
                failed.append({"input_key": key, "error": result["error"]})
                continue

            output_key = output_prefix + Path(result["output"]).name
            try:
                s3.upload_file(result["output"], bucket, output_key)
            except Exception as exc:
                failed.append({"input_key": key, "error": f"upload failed: {exc}"})
                continue

            pages.append({
                "input_key": key,
                "output_key": output_key,
                "width": result["width"],
                "height": result["height"],
                "angle": result.get("angle"),
            })

    return {"pages": pages, "failed": failed}


if __name__ == "__main__":
    # Pay the model load before the first job arrives, not during it.
    pipeline()
    runpod.serverless.start({"handler": handler})
