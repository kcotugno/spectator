package exhibit

type Scene struct {
	Terminal *Terminal
	Window   Widget
}

func (s *Scene) Render() {
	c := make([]Cell, 0)

	for _, row := range s.Window.Render() {
		for _, col := range row {
			c = append(c, col)
		}
	}

	s.Terminal.WriteCells(c)
	s.Terminal.Render()
}
