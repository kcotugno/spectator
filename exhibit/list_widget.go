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
	Style Style

	blockLock  sync.Mutex
	block      Block
	attributes Attributes

	rightAlign bool
	border     bool

	listLock sync.Mutex
	list     [][]Cell

	listBuf [][]Cell
}

func (l ListWidget) Attributes() Attributes {
	return l.attributes
}

func (l *ListWidget) SetAttributes(a Attributes) {
	l.attributes = a
	l.recalculateCells()
}

func (l ListWidget) Size() image.Point {
	return l.block.Rect.Size()
}

func (l *ListWidget) SetSize(p image.Point) {
	l.block.SetSize(p)
}

func (l *ListWidget) Render() Block {
	l.blockLock.Lock()
	defer l.blockLock.Unlock()

	b := NewBlock(0, 0, 0, 0)
	b.Rect = l.block.Rect

	for k, v := range l.block.Cells {
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

func (l *ListWidget) SetBorder(b bool) {
	if l.border == b {
		return
	}

	l.border = b

	l.recalculateCells()
}

func (l *ListWidget) SetRightAlign(b bool) {
	if l.rightAlign == b {
		return
	}

	l.rightAlign = b

	l.recalculateCells()
}

func (l *ListWidget) recalculateCells() {
	l.listLock.Lock()
	defer l.listLock.Unlock()

	l.blockLock.Lock()
	size := l.block.Rect.Size()
	l.blockLock.Unlock()

	cells := make(map[image.Point]Cell)

	var i, bx, by int
	if l.border {
		size = size.Add(image.Point{2, 2})
		bx = 1
		by = 1
	}

	for x := 0; x < size.X; x++ {
		for y := 0; y < size.Y; y++ {
			c := Cell{Value: ' '}

			if l.border {
				cell, ok := l.borderCell(image.Point{x, y}, size)
				if ok {
					cells[image.Point{x, y}] = cell
					continue
				}
			}

			if y < len(l.list)+by {
				length := len(l.list[y-by])

				if l.rightAlign {
					i = (size.X - x - length - bx) * -1
				} else {
					i = x - bx
				}

				if i < length && i >= 0 {
					c = l.list[y-by][i]
				}

				cells[image.Point{x, y}] = c
			}
		}
	}

	l.blockLock.Lock()
	l.block.Cells = cells
	l.blockLock.Unlock()
}

func (l *ListWidget) borderCell(p image.Point, size image.Point) (Cell, bool) {
	c := Cell{}
	c.Attrs = l.Attributes()

	if p.X != 0 && p.X != size.X-1 && p.Y != 0 && p.Y != size.Y-1 {
		return c, false
	}

	if p.X == 0 && p.Y == 0 {
		c.Value = BorderRune(TopLeft, l.Style)
	} else if p.X == size.X-1 && p.Y == 0 {
		c.Value = BorderRune(TopRight, l.Style)
	} else if p.X == 0 && p.Y == size.Y-1 {
		c.Value = BorderRune(BottomLeft, l.Style)
	} else if p.X == size.X-1 && p.Y == size.Y-1 {
		c.Value = BorderRune(BottomRight, l.Style)
	} else if p.X == 0 || p.X == size.X-1 {
		c.Value = BorderRune(Vertical, l.Style)
	} else if p.Y == 0 || p.Y == size.Y-1 {
		c.Value = BorderRune(Horizontal, l.Style)
	}

	return c, true
}
