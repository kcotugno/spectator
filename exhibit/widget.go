package exhibit

import (
	"image"
)

type Border struct {
	Top    bool
	Bottom bool
	Left   bool
	Right  bool
}

type Widget interface {
	Render(image.Point) Block
	Size() image.Point
	SetSize(image.Point)
	Attributes() Attributes
	SetAttributes(Attributes)
}
