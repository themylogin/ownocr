#!/usr/bin/env python
"""PP-OCRv6 line segmentation: boxes + per-line recognition confidence.

The VL pipeline returns paragraph blocks and no confidence. This gives the line
boxes and per-line confidence. Its dictionary has no polytonic
Greek, so treat its own text as geometry, not as the answer.

Detection runs at a scale measured from the page, not at native resolution.
The DB detector has a preferred text height in pixels: below it lines break into
fragments, above it whole lines drop out. So a cheap probe pass measures the
median line height, and the real pass runs at the scale that puts that height on
target. Recognition always crops from the native image.

    python ocr_v6.py -i page.png -o v6_lines.json
"""
from __future__ import annotations

import argparse
import json
import os
from pathlib import Path

import cv2
import numpy as np

DEVICE = os.environ.get("VL_DEVICE", "gpu:0")
DET_MODEL = os.environ.get("V6_DET_MODEL", "PP-OCRv6_medium_det")
REC_MODEL = os.environ.get("V6_REC_MODEL", "PP-OCRv6_medium_rec")

TARGET_LINE_HEIGHT = float(os.environ.get("V6_TARGET_LINE_HEIGHT", "50"))
PROBE_SIDE = int(os.environ.get("V6_PROBE_SIDE", "2000"))
FALLBACK_PROBE_SIDE = int(os.environ.get("V6_FALLBACK_PROBE_SIDE", "1280"))
MIN_PROBE_BOXES = int(os.environ.get("V6_MIN_PROBE_BOXES", "5"))
REC_BATCH = int(os.environ.get("V6_REC_BATCH", "8"))
BOX_THRESH = os.environ.get("V6_BOX_THRESH")

# Reuse the probe boxes when the probe already ran close enough to the wanted
# scale; a second pass would only cost time.
SCALE_BAND = (0.75, 1.3)
SCALE_LIMITS = (0.3, 3.0)
# v6 detection config: shortest side >= 64, longest side <= 4000.
MIN_SIDE, MAX_SIDE = 64, 4000

_ocr = None


class AdaptiveOCR:
    """PP-OCRv6 detection and recognition as separate modules.

    The pipeline class hides the resize, so the two detection passes need the
    standalone modules.
    """

    def __init__(self, device: str = DEVICE):
        from paddleocr import TextDetection, TextRecognition
        self.det = TextDetection(model_name=DET_MODEL, device=device)
        self.rec = TextRecognition(model_name=REC_MODEL, device=device)
        self.det_kwargs = {"box_thresh": float(BOX_THRESH)} if BOX_THRESH else {}

    def detect(self, image):
        polys = []
        for r in self.det.predict(image, **self.det_kwargs):
            for poly in r["dt_polys"]:
                polys.append(np.asarray(poly, dtype=np.float32).reshape(-1, 2))
        return polys

    def recognize(self, crops):
        texts, scores = [], []
        for r in self.rec.predict(crops, batch_size=REC_BATCH):
            texts.append(r["rec_text"])
            scores.append(float(r["rec_score"]))
        return texts, scores


def build_ocr(device: str = DEVICE):
    """Load det+rec once; a worker reuses it for every page."""
    global _ocr
    if _ocr is None:
        _ocr = AdaptiveOCR(device)
    return _ocr


def _load(image_path):
    if isinstance(image_path, np.ndarray):
        return image_path
    image = cv2.imdecode(np.fromfile(str(image_path), dtype=np.uint8), cv2.IMREAD_COLOR)
    if image is None:
        raise ValueError(f"cannot identify image file {image_path}")
    return image


def _fit_scale(scale: float, height: int, width: int) -> float:
    scale = float(np.clip(scale, *SCALE_LIMITS))
    long_side, short_side = max(height, width), min(height, width)
    scale = min(scale, MAX_SIDE / long_side)
    return max(scale, MIN_SIDE / short_side)


def _resize(image, scale: float):
    if abs(scale - 1.0) < 1e-3:
        return image
    height, width = image.shape[:2]
    new_h, new_w = max(1, round(height * scale)), max(1, round(width * scale))
    interp = cv2.INTER_AREA if scale < 1.0 else cv2.INTER_CUBIC
    return cv2.resize(image, (new_w, new_h), interpolation=interp)


def _poly_height(poly) -> float:
    (_, _), (w, h), _ = cv2.minAreaRect(poly)
    return float(min(w, h))


def _order_quad(poly):
    if len(poly) == 4:
        return poly.astype(np.float32)
    box = cv2.boxPoints(cv2.minAreaRect(poly.astype(np.float32)))
    s, d = box.sum(axis=1), np.diff(box, axis=1).ravel()
    return np.array([box[np.argmin(s)], box[np.argmin(d)],
                     box[np.argmax(s)], box[np.argmax(d)]], dtype=np.float32)


def _crop(image, poly):
    """Perspective-rectified line crop, upright."""
    pts = _order_quad(poly)
    width = int(max(np.linalg.norm(pts[0] - pts[1]), np.linalg.norm(pts[2] - pts[3])))
    height = int(max(np.linalg.norm(pts[0] - pts[3]), np.linalg.norm(pts[1] - pts[2])))
    width, height = max(width, 1), max(height, 1)
    dst = np.array([[0, 0], [width, 0], [width, height], [0, height]], dtype=np.float32)
    crop = cv2.warpPerspective(image, cv2.getPerspectiveTransform(pts, dst),
                               (width, height), borderMode=cv2.BORDER_REPLICATE,
                               flags=cv2.INTER_CUBIC)
    if crop.shape[0] / crop.shape[1] >= 1.5:
        crop = np.rot90(crop)
    return np.ascontiguousarray(crop)


def _to_native(polys, scale: float, height: int, width: int):
    native = []
    for poly in polys:
        p = poly / scale
        p[:, 0] = np.clip(p[:, 0], 0, width - 1)
        p[:, 1] = np.clip(p[:, 1], 0, height - 1)
        native.append(p.astype(np.float32))
    return native


def _reading_order(polys, median_height):
    """Top to bottom, left to right within a row.

    The detector emits contours in its own order, which is close to
    bottom-to-top. Consumers join lines in the order they get them.
    """
    if not polys:
        return polys
    tolerance = max(1.0, (median_height or _poly_height(polys[0])) * 0.5)
    ordered = sorted(polys, key=lambda p: float(p[:, 1].mean()))
    rows, row = [], [ordered[0]]
    for poly in ordered[1:]:
        if float(poly[:, 1].mean()) - float(row[0][:, 1].mean()) > tolerance:
            rows.append(row)
            row = []
        row.append(poly)
    rows.append(row)
    return [p for r in rows for p in sorted(r, key=lambda p: float(p[:, 0].min()))]


def detect_adaptive(ocr, image, target_height: float = TARGET_LINE_HEIGHT):
    """Line polygons in native coordinates, plus how they were found.

    The probe pass is a ruler, not an answer: both detector failure modes keep
    box heights correct, so a fragmented or partial probe still measures the
    page.
    """
    height, width = image.shape[:2]
    probe_scale = _fit_scale(min(1.0, PROBE_SIDE / max(height, width)), height, width)
    polys = ocr.detect(_resize(image, probe_scale))

    if len(polys) < MIN_PROBE_BOXES:
        retry_scale = _fit_scale(min(1.0, FALLBACK_PROBE_SIDE / max(height, width)),
                                 height, width)
        if abs(retry_scale - probe_scale) > 1e-3:
            probe_scale = retry_scale
            polys = ocr.detect(_resize(image, probe_scale))

    info = {"input_size": [int(width), int(height)],
            "probe_scale": round(probe_scale, 4), "probe_boxes": len(polys),
            "target_line_height": target_height, "det_scale": round(probe_scale, 4),
            "median_line_height": None, "rescanned": False}
    if len(polys) < MIN_PROBE_BOXES:
        return _to_native(polys, probe_scale, height, width), info

    median_height = float(np.median([_poly_height(p) for p in polys])) / probe_scale
    info["median_line_height"] = round(median_height, 2)
    if median_height <= 0:
        return _to_native(polys, probe_scale, height, width), info

    det_scale = _fit_scale(target_height / median_height, height, width)
    if SCALE_BAND[0] <= probe_scale / det_scale <= SCALE_BAND[1]:
        return _to_native(polys, probe_scale, height, width), info

    rescanned = ocr.detect(_resize(image, det_scale))
    if not rescanned:
        return _to_native(polys, probe_scale, height, width), info

    info["det_scale"] = round(det_scale, 4)
    info["rescanned"] = True
    return _to_native(rescanned, det_scale, height, width), info


def predict(ocr, image_path, target_height: float = TARGET_LINE_HEIGHT):
    """Everything PP-OCRv6 produces for a page image.

    Returns (raw, lines, text): `raw` the det polys, rec texts and scores, with
    the scales the page was detected at under `adaptive_scale`; `lines` the
    flattened [{text, score, bbox}] view; `text` the recognised lines joined.
    All three run top to bottom, left to right within a row -- there is no
    cross-column reading order without a layout model.
    """
    image = _load(image_path)
    polys, info = detect_adaptive(ocr, image, target_height)
    polys = _reading_order(polys, info["median_line_height"])
    texts, scores = ocr.recognize([_crop(image, p) for p in polys]) if polys else ([], [])

    boxes = [[int(p[:, 0].min()), int(p[:, 1].min()),
              int(p[:, 0].max()), int(p[:, 1].max())] for p in polys]
    raw = {"dt_polys": [p.tolist() for p in polys],
           "rec_polys": [p.tolist() for p in polys],
           "rec_boxes": boxes,
           "rec_texts": texts,
           "rec_scores": scores,
           "adaptive_scale": info}

    lines = [{"text": t, "score": s, "bbox": b}
             for t, s, b in zip(texts, scores, boxes)]
    return raw, lines, "\n".join(l["text"] for l in lines)


def lines_for(ocr, image_path: str):
    """[{text, score, bbox}] in detection order."""
    return predict(ocr, image_path)[1]


def main(argv=None) -> int:
    ap = argparse.ArgumentParser(prog="ocr_v6")
    ap.add_argument("-i", "--input", required=True)
    ap.add_argument("-o", "--output", default="v6_lines.json")
    ap.add_argument("--device", default=DEVICE)
    ap.add_argument("--target-height", type=float, default=TARGET_LINE_HEIGHT)
    a = ap.parse_args(argv)
    lines = predict(build_ocr(a.device), a.input, a.target_height)[1]
    Path(a.output).write_text(json.dumps(lines, ensure_ascii=False, indent=2), encoding="utf-8")
    print(f"{len(lines)} lines -> {a.output}")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
