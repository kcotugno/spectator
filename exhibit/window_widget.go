package exhibit

type WindowWidget struct {
	attributes  Attributes
	constraints Constraints
	size        Size
	border      Border
	widgets     []Widget
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

func (w WindowWidget) Constraints() Constraints {
	return w.constraints
}

func (w *WindowWidget) SetConstraints(c Constraints) {
	w.constraints = c
}

func (w WindowWidget) Size() Size {
	return w.size
}

func (w *WindowWidget) SetSize(s Size) {
	w.size = s
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
