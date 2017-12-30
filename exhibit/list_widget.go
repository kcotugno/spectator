package exhibit

import (
	"image"
	"sync"
)

type ListEntry interface {
	String() string
	Attributes() Attributes
}

type ListWidget struct {
	blockLock sync.Mutex
	block     Block

	attributesLock sync.Mutex
	attributes     Attributes

	rightAlignLock sync.Mutex
	rightAlign     bool

	borderLock sync.Mutex
	border     Border

	listLock sync.Mutex
	list     [][]Cell

	listBuf [][]Cell
}

func (l *ListWidget) Attributes() Attributes {
	l.attributesLock.Lock()
	defer l.attributesLock.Unlock()

	return l.attributes
}

func (l *ListWidget) SetAttributes(a Attributes) {
	l.attributesLock.Lock()
	l.attributes = a
	l.attributesLock.Unlock()

	l.recalculateCells()
}

func (l *ListWidget) Size() image.Point {
	l.blockLock.Lock()
	defer l.blockLock.Unlock()

	return l.block.Rect.Size()
}

func (l *ListWidget) SetSize(p image.Point) {
	l.blockLock.Lock()
	defer l.blockLock.Unlock()

	l.block.SetSize(p)
}

func (l *ListWidget) Origin() image.Point {
	l.blockLock.Lock()
	defer l.blockLock.Unlock()

	return l.block.Rect.Min
}

func (l *ListWidget) SetOrigin(p image.Point) {
	l.blockLock.Lock()
	defer l.blockLock.Unlock()

	l.block.SetOrigin(p)
}

func (l *ListWidget) Render(origin image.Point) Block {
	l.blockLock.Lock()
	defer l.blockLock.Unlock()

	b := NewBlock(0, 0, 0, 0)
	b.Rect = l.block.Rect.Add(origin)

	for k, v := range l.block.Cells {
		k = k.Add(origin)
		b.Cells[k] = v
	}

	return b
}

func (l *ListWidget) AddEntry(entry ListEntry) {
	l.listLock.Lock()
	defer l.listLock.Unlock()

	if l.listBuf == nil {
		l.listBuf = make([][]Cell, 1)
	} else {
		l.listBuf = append(l.listBuf, []Cell{})
	}

	index := len(l.listBuf) - 1

	for _, r := range entry.String() {
		cell := Cell{}
		cell.Value = r
		cell.Attrs = entry.Attributes()

		l.listBuf[index] = append(l.listBuf[index], cell)
	}
}

func (l *ListWidget) Commit() {
	l.listLock.Lock()
	l.list = append([][]Cell{}, l.listBuf...)
	l.listLock.Unlock()
	l.listBuf = nil
	l.recalculateCells()
}

func (l *ListWidget) Border() Border {
	l.borderLock.Lock()
	defer l.borderLock.Unlock()

	return l.border
}
func (l *ListWidget) SetBorder(b Border) {
	l.borderLock.Lock()
	if l.border == b {
		return
	}

	l.border = b
	l.borderLock.Unlock()

	l.recalculateCells()
}

func (l *ListWidget) RightAlign() bool {
	l.rightAlignLock.Lock()
	defer l.rightAlignLock.Unlock()

	return l.rightAlign
}

func (l *ListWidget) SetRightAlign(b bool) {
	l.rightAlignLock.Lock()
	if l.rightAlign == b {
		return
	}

	l.rightAlign = b
	l.rightAlignLock.Unlock()

	l.recalculateCells()
}

func (l *ListWidget) recalculateCells() {
	l.listLock.Lock()
	defer l.listLock.Unlock()

	l.blockLock.Lock()
	rect := l.block.Rect
	l.blockLock.Unlock()

	origin := rect.Min
	size := rect.Size()

	rightAlign := l.RightAlign()

	border := l.Border()

	cells := make(map[image.Point]Cell)

	var i, bx, by int
	if border.Visible {
		size = size.Add(image.Point{2, 2})
		bx = 1
		by = 1
	}

	for x := 0; x < size.X; x++ {
		for y := 0; y < size.Y; y++ {
			c := Cell{Value: ' '}
			point := image.Pt(x, y).Add(origin)

			cell, ok := border.Cell(point, rect)
			if ok {
				cells[point] = cell
				continue
			}

			if y < len(l.list)+by {
				length := len(l.list[y-by])

				if rightAlign {
					i = (size.X - x - length - bx) * -1
				} else {
					i = x - bx
				}

				if i < length && i >= 0 {
					c = l.list[y-by][i]
				}

				cells[point] = c
			}
		}
	}

	l.blockLock.Lock()
	l.block.Cells = cells
	l.blockLock.Unlock()
}
