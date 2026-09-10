#!/usr/bin/env python
"""Bake the four PaddleOCR-VL-1.6 pipeline models into the image at build time.

The doc-parsing pipeline is: PP-DocLayoutV3 (layout) + PP-LCNet_x1_0_doc_ori
(orientation) + UVDoc (unwarp) + PaddleOCR-VL-1.6 (the 0.9B VLM). The first three
run in-process on paddle (the client); the VLM is served by vLLM. All are pulled
from the PaddlePaddle org on Hugging Face with the build's hf_token, so a running
worker needs no network for models.
"""
import os
import sys
from pathlib import Path

from huggingface_hub import snapshot_download

# dir name -> HF repo (dir name must match what PaddleX/the genai server expect)
MODELS = {
    "PaddleOCR-VL-1.6": "PaddlePaddle/PaddleOCR-VL-1.6",
    "PP-DocLayoutV3": "PaddlePaddle/PP-DocLayoutV3",
    "PP-LCNet_x1_0_doc_ori": "PaddlePaddle/PP-LCNet_x1_0_doc_ori",
    "UVDoc": "PaddlePaddle/UVDoc",
    # PP-OCRv6 (classic det+rec): line boxes + a real per-line confidence, which the
    # VL pipeline does not provide.
    "PP-OCRv6_medium_det": "PaddlePaddle/PP-OCRv6_medium_det",
    "PP-OCRv6_medium_rec": "PaddlePaddle/PP-OCRv6_medium_rec",
}
DEST = Path(os.environ.get("PADDLE_PDX_CACHE_HOME", "/opt/paddlex")) / "official_models"


def main() -> int:
    DEST.mkdir(parents=True, exist_ok=True)
    token = os.environ.get("HF_TOKEN") or None
    for name, repo in MODELS.items():
        out = DEST / name
        snapshot_download(repo_id=repo, local_dir=str(out), token=token)
        files = sorted(p.name for p in out.iterdir())
        assert files, f"{name}: nothing downloaded"
        print(f"[hf] {name} <- {repo}: {len(files)} files")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
