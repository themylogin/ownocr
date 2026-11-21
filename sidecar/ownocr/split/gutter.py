"""Finds the gutter of a book spread and reports where to cut it.

The gutter is the only tall vertical structure in a spread. A photo of an open
book shows it as a dark shadow band, and a photo of a flat book shows it as a
bright pale ridge. The energy map reads both, because it takes the absolute
deviation from a wide horizontal blur.
"""
from __future__ import annotations

from pathlib import Path

import cv2
import numpy as np

WORK_WIDTH = 800

# The kernel sizes below are pixel counts at WORK_WIDTH. Every image is resized
# to that width first, so they stay resolution independent.
BACKGROUND_WIDTH = 121
RIDGE_HEIGHT = 49
REFINE_HALFWIDTH = 25

SEARCH_LO = 0.30
SEARCH_HI = 0.70
ANGLE_LIMIT = 6.0
ANGLE_STEP = 0.25
BANDS = 20

# A gutter runs the whole height of a spread, so every band carries some of it.
# Strength alone does not separate a spread from a single page: the cover of
# book 2 scores 15.2 for prominence, above 19 of its 67 real spreads. Coverage
# separates them by 0.20 against 1.00.
MIN_COVERAGE = 0.5
MIN_PROMINENCE = 3.0
MIN_BANDS = 6

# Coverage alone lets a paragraph pass: the residue a block of text leaves behind
# covers every band that block covers. A gutter is also an absolute amount of
# contrast, so this floor counts grey levels per row of the work image. It is not
# a ratio, because the strongest column of a page is often a dark scanner border
# rather than its gutter.
#
# Measured over 181 pages of four books. The three false gutters score 0.129 to
# 0.164. The weakest true gutter whose page the middle column cannot cut scores
# 2.199. This floor sits between them.
MIN_ENERGY_PER_ROW = 0.5

OVERLAP = 0.005

# A spread is wider than it is tall. The threshold is not `width > height`: a
# page photographed with a little of the desk on either side is wider by a few
# percent, and cutting that one in two would be wrong. It is not 1.2 either: a
# book whose pages are 0.6 times as wide as they are tall scans its spreads at
# 2904 by 2423, which is 1.1985.
LANDSCAPE_RATIO = 1.15


class UnreadableImage(Exception):
    def __init__(self, path: str):
        super().__init__(path)
        self.path = path


def cut_columns(path: str | Path) -> tuple[int, int] | None:
    """Returns the two cut columns of a spread, or None for a single page."""
    image = cv2.imread(str(path), cv2.IMREAD_COLOR)
    if image is None:
        raise UnreadableImage(str(path))
    return columns(image)


def columns(image: np.ndarray) -> tuple[int, int] | None:
    height, width = image.shape[:2]
    curve = _gutter(image)
    if curve is None:
        return _middle(width, height)

    positions = np.polyval(curve, np.arange(height, dtype=float))
    margin = round(OVERLAP * width)

    # A crop takes a column, but the gutter is a curve. Cutting at its leftmost
    # point would clip the left page on every lower row, so each half runs to
    # the far extreme and the two halves overlap.
    right = _clamp(int(np.floor(positions.min())) - margin, width)
    left = _clamp(int(np.ceil(positions.max())) + margin, width)
    return max(left, right), right


def _clamp(value: int, width: int) -> int:
    return max(0, min(value, width))


# A landscape image the detector finds no gutter in still holds two pages: its
# shape says so. Both numbers are the middle column, because there is no curve
# to follow here, and a margin would only put one strip in both pages.
def _middle(width: int, height: int) -> tuple[int, int] | None:
    if width < LANDSCAPE_RATIO * height:
        return None
    half = width // 2
    return half, half


def _gutter(image: np.ndarray):
    height, width = image.shape[:2]
    scale = width / float(WORK_WIDTH)
    work = cv2.resize(image, (WORK_WIDTH, max(1, round(height / scale))),
                      interpolation=cv2.INTER_AREA)
    gray = cv2.cvtColor(work, cv2.COLOR_BGR2GRAY)

    energy = _energy(gray)
    angle, centre, prominence, peak = _coarse_line(energy)
    points = _band_centroids(energy, angle, centre)

    if len(points) / float(BANDS) < MIN_COVERAGE or len(points) < MIN_BANDS:
        return None
    if prominence < MIN_PROMINENCE:
        return None
    if peak / energy.shape[0] < MIN_ENERGY_PER_ROW:
        return None

    curve = _fit(points)
    if curve is None:
        return None

    a, b, c = curve
    return a / scale, b, c * scale


# The opening deletes every letter, because text is short and horizontal, and
# keeps the gutter, which is tall.
def _energy(gray: np.ndarray) -> np.ndarray:
    blurred = cv2.GaussianBlur(gray, (0, 0), 1.2).astype(np.float32)
    deviation = np.abs(cv2.blur(blurred, (BACKGROUND_WIDTH, 1)) - blurred)
    kernel = cv2.getStructuringElement(cv2.MORPH_RECT, (1, RIDGE_HEIGHT))
    return cv2.morphologyEx(deviation, cv2.MORPH_OPEN, kernel)


def _coarse_line(energy: np.ndarray) -> tuple[float, int, float, float]:
    height, width = energy.shape
    lo, hi = int(SEARCH_LO * width), int(SEARCH_HI * width)
    centre = (width / 2.0, height / 2.0)
    best_peak, best_angle, best_x, best_profile = -1.0, 0.0, lo, None

    for angle in np.arange(-ANGLE_LIMIT, ANGLE_LIMIT + 1e-9, ANGLE_STEP):
        matrix = cv2.getRotationMatrix2D(centre, float(angle), 1.0)
        rotated = cv2.warpAffine(energy, matrix, (width, height),
                                 flags=cv2.INTER_LINEAR, borderValue=0)
        profile = cv2.GaussianBlur(rotated.sum(axis=0).reshape(1, -1), (0, 0), 2).ravel()
        band = profile[lo:hi]
        peak = float(band.max())
        if peak > best_peak:
            best_peak, best_angle = peak, float(angle)
            best_x, best_profile = int(np.argmax(band)) + lo, profile

    band = best_profile[lo:hi]
    return best_angle, best_x, float(band.max() / (np.median(band) + 1e-6)), best_peak


def _band_centroids(energy: np.ndarray, angle: float, x_centre: int) -> list:
    height, width = energy.shape
    shift = np.tan(np.radians(-angle))
    points = []

    for index in range(BANDS):
        top, bottom = int(index * height / BANDS), int((index + 1) * height / BANDS)
        y = (top + bottom) / 2.0
        expected = x_centre + shift * (y - height / 2.0)
        lo = int(max(0, expected - REFINE_HALFWIDTH))
        hi = int(min(width, expected + REFINE_HALFWIDTH))
        if hi - lo < 8:
            continue
        profile = energy[top:bottom, lo:hi].sum(axis=0)
        profile = np.clip(profile - profile.min(), 0, None)
        total = profile.sum()
        if total <= 0:
            continue
        columns_here = np.arange(lo, hi, dtype=float)
        points.append((y, float((profile * columns_here).sum() / total), float(total)))

    return points


# The gutter bows by up to 2 percent of the width, so a quadratic fits it where
# a straight line does not.
def _fit(points: list):
    ys = np.array([p[0] for p in points])
    xs = np.array([p[1] for p in points])
    weights = np.array([p[2] for p in points])

    keep = weights > 0.2 * np.median(weights)
    if keep.sum() < MIN_BANDS:
        keep = np.ones(len(points), dtype=bool)

    for _ in range(4):
        if keep.sum() < MIN_BANDS:
            break
        line = np.polyfit(ys[keep], xs[keep], 1, w=weights[keep])
        distance = np.abs(xs - np.polyval(line, ys))
        keep = distance < max(2.5 * np.median(distance[keep]), 2.0)

    if keep.sum() < MIN_BANDS:
        return None
    return np.polyfit(ys[keep], xs[keep], 2, w=weights[keep])
