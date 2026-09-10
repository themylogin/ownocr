package books

import "testing"

func pagesOf(sizes ...[2]int) []Page {
	pages := make([]Page, 0, len(sizes))
	for i, size := range sizes {
		pages = append(pages, Page{PageIndex: i + 1, Width: size[0], Height: size[1]})
	}
	return pages
}

func TestComputeGridCell(t *testing.T) {
	cases := []struct {
		name       string
		pages      []Page
		wantWidth  int
		wantHeight int
	}{
		{
			name:      "all landscape 3:2",
			pages:     pagesOf([2]int{3000, 2000}, [2]int{3000, 2000}, [2]int{3000, 2000}),
			wantWidth: 480, wantHeight: 320,
		},
		{
			name:      "all portrait 2:3",
			pages:     pagesOf([2]int{2000, 3000}, [2]int{2000, 3000}),
			wantWidth: 320, wantHeight: 480,
		},
		{
			name:      "mixed orientations",
			pages:     pagesOf([2]int{3000, 2000}, [2]int{2000, 3000}),
			wantWidth: 480, wantHeight: 480,
		},
		{
			name:      "one square page forces a square cell",
			pages:     pagesOf([2]int{2000, 3000}, [2]int{1000, 1000}),
			wantWidth: 480, wantHeight: 480,
		},
		{
			name:      "all square",
			pages:     pagesOf([2]int{1000, 1000}, [2]int{800, 800}),
			wantWidth: 480, wantHeight: 480,
		},
		{
			name:      "extreme landscape clamps the short side",
			pages:     pagesOf([2]int{3000, 500}, [2]int{3000, 500}),
			wantWidth: 480, wantHeight: 240,
		},
		{
			name:      "extreme portrait clamps the short side",
			pages:     pagesOf([2]int{500, 3000}, [2]int{500, 3000}),
			wantWidth: 240, wantHeight: 480,
		},
		{
			name:      "single portrait page",
			pages:     pagesOf([2]int{2000, 3000}),
			wantWidth: 320, wantHeight: 480,
		},
		{
			name:      "single landscape page",
			pages:     pagesOf([2]int{3000, 2000}),
			wantWidth: 480, wantHeight: 320,
		},
		{
			name:      "no pages",
			pages:     nil,
			wantWidth: 480, wantHeight: 480,
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := ComputeGridCell(c.pages)

			if got.Width != c.wantWidth || got.Height != c.wantHeight {
				t.Errorf("ComputeGridCell = %dx%d, want %dx%d",
					got.Width, got.Height, c.wantWidth, c.wantHeight)
			}
		})
	}
}

// The median keeps one odd page from skewing the grid for the whole book.
func TestComputeGridCellUsesTheMedian(t *testing.T) {
	pages := pagesOf(
		[2]int{2000, 3000},
		[2]int{2000, 3000},
		[2]int{2000, 3000},
		[2]int{100, 3000},
	)

	got := ComputeGridCell(pages)

	if got.Width != 320 || got.Height != 480 {
		t.Errorf("ComputeGridCell = %dx%d, want 320x480", got.Width, got.Height)
	}
}

func TestComputeGridCellLongestSideIsAlways480(t *testing.T) {
	sets := [][]Page{
		pagesOf([2]int{3000, 2000}),
		pagesOf([2]int{2000, 3000}),
		pagesOf([2]int{1000, 1000}),
		pagesOf([2]int{9000, 100}),
		pagesOf([2]int{100, 9000}),
	}

	for _, pages := range sets {
		got := ComputeGridCell(pages)

		if got.Width != 480 && got.Height != 480 {
			t.Errorf("ComputeGridCell = %dx%d has no 480 side", got.Width, got.Height)
		}
		if got.Width > 480 || got.Height > 480 {
			t.Errorf("ComputeGridCell = %dx%d exceeds 480", got.Width, got.Height)
		}
		if got.Width < 240 || got.Height < 240 {
			t.Errorf("ComputeGridCell = %dx%d is below the 240 clamp", got.Width, got.Height)
		}
	}
}

func TestComputeGridCellIgnoresZeroDimensions(t *testing.T) {
	got := ComputeGridCell(pagesOf([2]int{0, 0}))

	if got.Width != 480 || got.Height != 480 {
		t.Errorf("ComputeGridCell = %dx%d, want 480x480", got.Width, got.Height)
	}
}
