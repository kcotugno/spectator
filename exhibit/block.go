package exhibit

import (
	"image"
)

type Block struct {
	Rect image.Rectangle
	Cells map[image.Point]Cell
}

func NewBlock() Block {
	b := Block{}
	b.Cells := make(map[image.Point]Cell)
	return b
}
