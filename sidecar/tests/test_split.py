import cv2
import numpy as np
import pytest
from fastapi.testclient import TestClient

from ownocr import split
from ownocr.app import app

WIDTH, HEIGHT = 1200, 900
SINGLE_WIDTH, SINGLE_HEIGHT = 900, 1200
PAPER = 235
INK = 40
GUTTER = 90

client = TestClient(app)


def page(gutter_at=None, gutter_rows=None, lean=0):
    """A light page with two blocks of text rows and an optional dark gutter."""
    image = np.full((HEIGHT, WIDTH, 3), PAPER, np.uint8)
    for start, end in ((80, 520), (680, 1120)):
        for y in range(120, HEIGHT - 120, 24):
            image[y:y + 8, start:end] = INK

    if gutter_at is None:
        return image

    rows = gutter_rows if gutter_rows is not None else range(HEIGHT)
    for y in rows:
        centre = gutter_at + round(lean * (y - HEIGHT / 2.0))
        image[y, max(0, centre - 6):centre + 6] = GUTTER
    return image


# A page that is taller than it is wide, which the landscape rule leaves whole.
# The canvas of `page` is 1200 by 900, so the rule reads it as a spread whatever
# is drawn on it.
def single(band_rows=None):
    """A portrait page of one block of text rows, and an optional dark band."""
    image = np.full((SINGLE_HEIGHT, SINGLE_WIDTH, 3), PAPER, np.uint8)
    for y in range(120, SINGLE_HEIGHT - 120, 24):
        image[y:y + 8, 80:SINGLE_WIDTH - 80] = INK

    if band_rows is None:
        return image

    for y in band_rows:
        image[y, SINGLE_WIDTH // 2 - 6:SINGLE_WIDTH // 2 + 6] = GUTTER
    return image


def blank(width, height):
    """A page with nothing on it, so the detector can find no gutter."""
    return np.full((height, width, 3), PAPER, np.uint8)


def write(tmp_path, name, image):
    path = tmp_path / name
    assert cv2.imwrite(str(path), image)
    return str(path)


def test_spread_returns_a_pair():
    assert split.columns(page(gutter_at=WIDTH // 2)) is not None


def test_single_page_returns_none():
    assert split.columns(single()) is None


def test_columns_overlap_and_lie_inside_the_image():
    left, right = split.columns(page(gutter_at=WIDTH // 2))
    assert left >= right
    assert 0 <= right <= WIDTH
    assert 0 <= left <= WIDTH


def test_the_gutter_is_found_where_it_was_drawn():
    left, right = split.columns(page(gutter_at=WIDTH // 2))
    assert right <= WIDTH // 2 <= left


def test_a_leaning_gutter_widens_the_gap():
    upright = split.columns(page(gutter_at=WIDTH // 2))
    leaning = split.columns(page(gutter_at=WIDTH // 2, lean=0.08))
    assert leaning is not None
    assert leaning[0] - leaning[1] > upright[0] - upright[1]
    assert leaning[0] >= leaning[1]


# The cover of a real book scores high on gutter strength but carries that
# strength over a fraction of its height. Coverage is what rejects it.
def test_a_strong_mark_over_part_of_the_height_is_not_a_gutter():
    partial = single(band_rows=range(SINGLE_HEIGHT // 3))
    assert split.columns(partial) is None


# A landscape image holds two pages whatever the detector makes of its gutter,
# so a miss answers the middle rather than nothing.
def test_a_landscape_image_with_no_gutter_is_cut_at_its_middle():
    assert split.columns(blank(1600, 1200)) == (800, 800)


def test_the_two_middle_pages_meet():
    width = 1600
    left, right = split.columns(blank(width, 1200))

    assert left == right
    assert left + (width - right) == width


def test_a_found_gutter_beats_the_middle():
    off_centre = round(WIDTH * 0.4)
    left, right = split.columns(page(gutter_at=off_centre))

    assert right <= off_centre <= left
    assert left < WIDTH // 2


def test_an_image_just_wide_enough_is_cut():
    assert split.columns(blank(1200, 1000)) == (600, 600)


def test_an_image_not_quite_wide_enough_is_whole():
    assert split.columns(blank(1100, 1000)) is None


def test_a_square_image_is_whole():
    assert split.columns(blank(1000, 1000)) is None


def test_a_portrait_image_with_no_gutter_is_whole():
    assert split.columns(blank(1200, 1600)) is None


def test_cut_columns_refuses_a_missing_path(tmp_path):
    with pytest.raises(split.UnreadableImage):
        split.cut_columns(tmp_path / "absent.jpg")


def test_split_answers_one_entry_per_image(tmp_path):
    whole = write(tmp_path, "single.jpg", single())
    spread = write(tmp_path, "spread.jpg", page(gutter_at=WIDTH // 2))

    response = client.post("/split", json=[whole, spread])

    assert response.status_code == 200
    answer = response.json()
    assert len(answer) == 2
    assert answer[0] is None
    assert len(answer[1]) == 2
    assert all(isinstance(column, int) for column in answer[1])


def test_split_keeps_the_order(tmp_path):
    paths, spreads = [], []
    for index in range(20):
        is_spread = index % 3 == 0
        image = page(gutter_at=WIDTH // 2) if is_spread else single()
        paths.append(write(tmp_path, f"page{index:02d}.jpg", image))
        spreads.append(is_spread)

    answer = client.post("/split", json=paths).json()

    assert len(answer) == 20
    assert [entry is not None for entry in answer] == spreads


def test_split_answers_an_empty_list():
    assert client.post("/split", json=[]).json() == []


def test_split_refuses_a_missing_path(tmp_path):
    real = write(tmp_path, "real.jpg", page(gutter_at=WIDTH // 2))
    missing = str(tmp_path / "absent.jpg")

    response = client.post("/split", json=[real, missing])

    assert response.status_code == 400
    assert missing in response.json()["detail"]


def test_split_refuses_a_corrupt_image(tmp_path):
    corrupt = tmp_path / "corrupt.jpg"
    corrupt.write_bytes(b"\xff\xd8\xff\xe0 not an image")
    real = write(tmp_path, "real.jpg", page(gutter_at=WIDTH // 2))

    response = client.post("/split", json=[real, str(corrupt)])

    assert response.status_code == 400
    assert str(corrupt) in response.json()["detail"]
