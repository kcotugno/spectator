package exhibit

import (
	"image"
)

type Scene struct {
	Terminal *Terminal
	Window   Widget
}

func (s *Scene) Render() {
	s.Window.SetSize(s.Terminal.Size())

	c := make([]Cell, 0)

	for k, v := range s.Window.Render(image.Point{}).Cells {
		v.Point = k
		c = append(c, v)
	}

	s.Terminal.WriteCells(c)
	s.Terminal.Render()
}
