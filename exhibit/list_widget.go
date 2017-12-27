package exhibit

import (
	"sync"
)

type ListWidget struct {
	Constraints Constraints
	Attrs       Attributes

	cellLock sync.Mutex
	cells    [][]Cell

	rightAlign bool
	border     bool

	listLock sync.Mutex
	list     [][]rune

	lastSize Size
}

func (l *ListWidget) Render() [][]Cell {
	l.cellLock.Lock()
	defer l.cellLock.Unlock()

	var sx int
	sy := len(l.cells)

	if sy > 0 {
		sx = len(l.cells[0])
	} else {
		return make([][]Cell, 0)
	}

	dx := 0
	dy := 0
	if l.lastSize.X > sx {
		dx = l.lastSize.X - sx
	}

	if l.lastSize.Y > sy {
		dy = l.lastSize.Y - sy
	}

	for y := 0; y < sy+dy; y++ {
		if y >= sy {
			l.cells = append(l.cells, []Cell{})
		}

		for x := 0; x < sx+dx; x++ {
			if x >= sx || y >= sy {
				c := Cell{}
				c.Pos.X = x
				c.Pos.Y = y
				c.Value = ' '
				l.cells[y] = append(l.cells[y], c)
			} else {
				l.cells[y][x].Attrs = l.Attrs
				l.cells[y][x].Pos.X = x
				l.cells[y][x].Pos.Y = y
			}
		}
	}

	l.lastSize.X = sx
	l.lastSize.Y = sy
	return append([][]Cell(nil), l.cells...)
}

func (l *ListWidget) SetList(list []string) {
	buf := make([][]rune, len(list))

	for i, s := range list {
		buf[i] = make([]rune, 0)

		for _, r := range s {
			buf[i] = append(buf[i], r)
		}
	}

	l.listLock.Lock()
	l.list = buf
	l.listLock.Unlock()

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

	var longest int
	var border int

	for _, s := range l.list {
		if longest < len(s) {
			longest = len(s)
		}
	}

	cells := make([][]Cell, len(l.list))

	if l.border {
		border = 1

		top := make([]Cell, longest+2)
		top[0] = Cell{Value: '┏'}
		top[longest+1] = Cell{Value: '┓'}
		for i := 1; i <= longest; i++ {
			top[i] = Cell{Value: '━'}
		}

		bottom := append([]Cell(nil), top...)
		bottom[0] = Cell{Value: '┗'}
		bottom[longest+1] = Cell{Value: '┛'}

		cells = append([][]Cell{top}, cells...)
		cells = append(cells, bottom)
	}

	for i, s := range l.list {
		cells[i+border] = make([]Cell, longest+border+border)

		var start int
		if l.rightAlign {
			start = longest - len(s)
		} else {
			start = 0
		}

		if l.border {
			c := Cell{Value: '┃'}
			cells[i+border][0] = c
			cells[i+border][longest+1] = c
		}

		for j := 0; j < longest; j++ {
			c := Cell{}
			if j > start+len(s)-1 || j < start {
				c.Value = ' '
			} else {
				c.Value = s[j-start]
			}

			cells[i+border][j+border] = c
		}
	}

	l.cellLock.Lock()
	defer l.cellLock.Unlock()

	l.cells = cells
}
