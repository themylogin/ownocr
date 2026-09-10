#!/usr/bin/env python
"""PP-OCRv6 line segmentation: boxes + per-line recognition confidence.

The VL pipeline returns paragraph blocks and no confidence. This gives the line
boxes and per-line confidence. Its dictionary has no polytonic
Greek, so treat its own text as geometry, not as the answer.

    python ocr_v6.py -i page.png -o v6_lines.json
"""
from __future__ import annotations

import argparse
import json
import os
from pathlib import Path

DEVICE = os.environ.get("VL_DEVICE", "gpu:0")
_ocr = None


def build_ocr(device: str = DEVICE):
    """Load det+rec once; a worker reuses it for every page."""
    global _ocr
    if _ocr is None:
        from paddleocr import PaddleOCR
        _ocr = PaddleOCR(
            use_doc_orientation_classify=False,   # the VL pipeline already handles
            use_doc_unwarping=False,              # orientation/unwarping upstream
            use_textline_orientation=False,
            device=device,
        )
    return _ocr


def _raw(res):
    j = res.json() if callable(getattr(res, "json", None)) else getattr(res, "json", None)
    if isinstance(j, dict) and "res" in j:
        j = j["res"]
    return j


def predict(ocr, image_path: str):
    """Everything PP-OCRv6 produces for a page.

    Returns (raw, lines, text): `raw` is the pipeline result verbatim (det polys,
    rec texts, scores, boxes, model settings...), `lines` the flattened
    [{text, score, bbox}] view, `text` the recognised lines joined in detection
    order -- there is no cross-column reading order without a layout model.
    """
    raw, lines = [], []
    for r in ocr.predict(str(image_path)):
        raw.append(_raw(r))
        for t, s, poly in zip(r["rec_texts"], r["rec_scores"], r["rec_polys"]):
            xs = [int(p[0]) for p in poly]
            ys = [int(p[1]) for p in poly]
            lines.append({"text": t, "score": float(s),
                          "bbox": [min(xs), min(ys), max(xs), max(ys)]})
    return (raw[0] if len(raw) == 1 else {"pages": raw}), lines, "\n".join(l["text"] for l in lines)


def lines_for(ocr, image_path: str):
    """[{text, score, bbox}] in detection order."""
    return predict(ocr, image_path)[1]


def main(argv=None) -> int:
    ap = argparse.ArgumentParser(prog="ocr_v6")
    ap.add_argument("-i", "--input", required=True)
    ap.add_argument("-o", "--output", default="v6_lines.json")
    ap.add_argument("--device", default=DEVICE)
    a = ap.parse_args(argv)
    lines = lines_for(build_ocr(a.device), a.input)
    Path(a.output).write_text(json.dumps(lines, ensure_ascii=False, indent=2), encoding="utf-8")
    print(f"{len(lines)} lines -> {a.output}")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
