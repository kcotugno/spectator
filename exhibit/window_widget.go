package exhibit

type WindowWidget struct {
	Constraints Constraints
	Border      Border

	widgets []Widget
}

func (w *WindowWidget) AddWidget(widget Widget) {
	if w.widgets == nil {
		w.widgets = make([]Widget, 0)
	}

	w.widgets = append(w.widgets, widget)
}

func (w *WindowWidget) Render() [][]Cell {
	c := make([][]Cell, 0)

	var y int
	for _, w := range w.widgets {
		for _, row := range w.Render() {
			t := make([]Cell, len(row))
			c = append(c, t)

			for j, col := range row {
				col.Pos.Y = y
				c[y][j] = col
			}

			y++
		}
	}

	return c
}
