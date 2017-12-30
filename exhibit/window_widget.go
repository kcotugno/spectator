package exhibit

import (
	"image"
)

type WindowWidget struct {
	block      Block
	attributes Attributes
	border     Border
	widgets    []Widget
}

func (w *WindowWidget) AddWidget(widget Widget) {
	if w.widgets == nil {
		w.widgets = make([]Widget, 0)
	}

	w.widgets = append(w.widgets, widget)
}

func (w WindowWidget) Attributes() Attributes {
	return w.attributes
}

func (w *WindowWidget) SetAttributes(a Attributes) {
	w.attributes = a
}

func (w WindowWidget) Size() image.Point {
	return w.block.Rect.Size()
}

func (w *WindowWidget) SetSize(p image.Point) {
	w.block.SetSize(p)
}

func (w *WindowWidget) Render() Block {
	if w.block.Rect.Size().X == 0 || w.block.Rect.Size().Y == 0 {
		return w.renderContent()
	} else {
		return w.renderSize()
	}
}

func (w *WindowWidget) renderContent() Block {
	//         c := make([][]Cell, 0)

	//         var y int

	//         for _, w := range w.widgets {
	//                 for _, row := range w.Render() {

	//                         t := make([]Cell, len(row))
	//                         c = append(c, t)

	//                         for j, col := range row {
	//                                 col.Pos.Y = y
	//                                 c[y][j] = col
	//                         }

	//                         y++
	//                 }
	//         }

	return Block{}
}

func (w *WindowWidget) renderSize() Block {
	//         c := make([][]Cell, w.size.Y)

	//         for y := 0; y < w.size.Y; y++ {
	//                 for x := 0; x < w.size.X; x++ {
	//                         c[y][x] = Cell{Pos: Position{X: x, Y: y}}
	//                 }
	//         }

	//         return c
	return Block{}
}
