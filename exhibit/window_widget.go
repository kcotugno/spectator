package exhibit

import (
	"image"
	"sync"
)

type WindowWidget struct {
	Style     Style
	blockLock sync.Mutex
	block     Block

	attributesLock sync.Mutex
	attributes     Attributes

	borderLock sync.Mutex
	border     Border

	widgetLock sync.Mutex
	widgets    []Widget
}

func (w *WindowWidget) AddWidget(widget Widget) {
	w.widgetLock.Lock()
	defer w.widgetLock.Unlock()

	if w.widgets == nil {
		w.widgets = make([]Widget, 0)
	}

	w.widgets = append(w.widgets, widget)
}

func (w WindowWidget) Attributes() Attributes {
	w.attributesLock.Lock()
	defer w.attributesLock.Unlock()

	return w.attributes
}

func (w *WindowWidget) SetAttributes(a Attributes) {
	w.attributesLock.Lock()
	defer w.attributesLock.Unlock()

	w.attributes = a
}

func (w WindowWidget) Size() image.Point {
	return w.block.Rect.Size()
}

func (w *WindowWidget) SetSize(p image.Point) {
	w.block.SetSize(p)
}

func (w *WindowWidget) Origin() image.Point {
	w.blockLock.Lock()
	defer w.blockLock.Unlock()

	return w.block.Rect.Min
}

func (w *WindowWidget) SetOrigin(p image.Point) {
	w.blockLock.Lock()
	defer w.blockLock.Unlock()

	w.block.SetOrigin(p)
}

func (w *WindowWidget) Border() Border {
	w.borderLock.Lock()
	defer w.borderLock.Unlock()

	return w.border
}

func (w *WindowWidget) SetBorder(b Border) {
	w.borderLock.Lock()
	defer w.borderLock.Unlock()

	w.border = b
}

func (w *WindowWidget) Render(origin image.Point) Block {
	if w.block.Rect.Size().X == 0 || w.block.Rect.Size().Y == 0 {
		return NewBlock(0, 0, 0, 0)
	}

	w.widgetLock.Lock()
	defer w.widgetLock.Unlock()

	w.blockLock.Lock()
	defer w.blockLock.Unlock()

	var borderAdj image.Point
	border := w.Border()
	if border.Visible {
		borderAdj = image.Pt(1, 1)
	}

	block := NewBlock(0, 0, 0, 0)
	block.SetSize(w.block.Rect.Size())
	block.SetOrigin(origin.Add(w.block.Origin()))

	for _, widget := range w.widgets {
		cells := widget.Render(block.Origin().Add(borderAdj)).Cells

		for k, v := range cells {
			if !k.In(block.Rect) {
				continue
			}

			block.Cells[k] = v
		}
	}

	if border.Visible {
		for x := 0; x < block.Size().X; x++ {
			for y := 0; y < block.Size().Y; y++ {
				point := image.Pt(x, y).Add(block.Origin())
				c, ok := border.Cell(point, block.Rect)
				if ok {
					block.Cells[point] = c
				}
			}
		}
	}

	return block
}
