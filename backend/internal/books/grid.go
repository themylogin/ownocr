package books

import "sort"

const (
	gridLongSide    = 480
	gridMinShort    = 240
	defaultGridSide = gridLongSide
)

type GridCell struct {
	Width  int `json:"width"`
	Height int `json:"height"`
}

// ComputeGridCell derives one cell size for a whole book. The longest side is
// always 480 px. A mixed or square book gets a square cell, because no single
// rectangle fits both orientations without cropping.
func ComputeGridCell(pages []Page) GridCell {
	if len(pages) == 0 {
		return GridCell{Width: defaultGridSide, Height: defaultGridSide}
	}

	aspects := make([]float64, 0, len(pages))
	landscape, portrait, square := 0, 0, 0

	for _, page := range pages {
		if page.Width <= 0 || page.Height <= 0 {
			square++
			continue
		}

		aspect := float64(page.Width) / float64(page.Height)
		aspects = append(aspects, aspect)

		switch {
		case page.Width > page.Height:
			landscape++
		case page.Width < page.Height:
			portrait++
		default:
			square++
		}
	}

	if square > 0 || (landscape > 0 && portrait > 0) || len(aspects) == 0 {
		return GridCell{Width: gridLongSide, Height: gridLongSide}
	}

	median := medianOf(aspects)

	if landscape > 0 {
		return GridCell{Width: gridLongSide, Height: clampShort(round(gridLongSide / median))}
	}
	return GridCell{Width: clampShort(round(gridLongSide * median)), Height: gridLongSide}
}

func medianOf(values []float64) float64 {
	sorted := append([]float64(nil), values...)
	sort.Float64s(sorted)

	middle := len(sorted) / 2
	if len(sorted)%2 == 1 {
		return sorted[middle]
	}
	return (sorted[middle-1] + sorted[middle]) / 2
}

func round(value float64) int {
	return int(value + 0.5)
}

func clampShort(value int) int {
	if value < gridMinShort {
		return gridMinShort
	}
	if value > gridLongSide {
		return gridLongSide
	}
	return value
}
