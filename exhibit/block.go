package exhibit

import (
	"image"
)

type Block struct {
	Rect  image.Rectangle
	Cells map[image.Point]Cell
}

func NewBlock(originx, originy, sizex, sizey int) Block {
	b := Block{}
	b.Rect = image.Rect(originx, originy, originx+sizex, originy+sizey)
	b.Cells = make(map[image.Point]Cell)
	return b
}

func (b Block) Size() image.Point {
	return b.Rect.Size()
}

func (b *Block) SetSize(p image.Point) {
	dx := b.Rect.Min.X
	dy := b.Rect.Min.Y
	b.Rect.Max.X = p.X + dx
	b.Rect.Max.Y = p.Y + dy
}

func (b Block) Origin() image.Point {
	return b.Rect.Min
}

func (b *Block) SetOrigin(p image.Point) {
	d := p.Sub(b.Rect.Min)
	b.Rect = b.Rect.Add(d)
}
