import os
import sys
import tarfile
import tempfile
import shutil
from pathlib import Path

import requests
from huggingface_hub import snapshot_download

MODELS = ["PP-LCNet_x1_0_doc_ori", "UVDoc"]
DEST = Path(os.environ.get("PADDLE_PDX_CACHE_HOME", "/opt/paddlex")) / "official_models"
BOS = "https://paddle-model-ecology.bj.bcebos.com/paddlex/official_inference_model/paddle3.0.0"


def from_hf(name: str, out: Path) -> None:
    snapshot_download(
        repo_id=f"PaddlePaddle/{name}",
        local_dir=str(out),
        token=os.environ.get("HF_TOKEN") or None,
    )


def from_bos(name: str, out: Path) -> None:
    url = f"{BOS}/{name}_infer.tar"
    with tempfile.TemporaryDirectory() as td:
        tar = Path(td) / "m.tar"
        with requests.get(url, stream=True, timeout=120) as r:
            r.raise_for_status()
            with open(tar, "wb") as f:
                for chunk in r.iter_content(1 << 20):
                    f.write(chunk)
        with tarfile.open(tar) as t:
            t.extractall(td)
        src = Path(td) / name
        if not src.is_dir():
            src = next(p for p in Path(td).iterdir() if p.is_dir() and p.name != "m.tar")
        shutil.copytree(src, out, dirs_exist_ok=True)


def main() -> int:
    DEST.mkdir(parents=True, exist_ok=True)
    for name in MODELS:
        out = DEST / name
        try:
            from_hf(name, out)
            src = "huggingface"
        except Exception as exc:
            print(f"HF download of {name} failed ({exc!r}); falling back to BOS", file=sys.stderr)
            shutil.rmtree(out, ignore_errors=True)
            from_bos(name, out)
            src = "bos"
        files = sorted(p.name for p in out.iterdir())
        assert any(f.startswith("inference.") for f in files), f"{name}: no inference files, got {files}"
        print(f"[{src}] {name}: {files}")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
