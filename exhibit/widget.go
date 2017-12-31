package exhibit

import (
	"image"
)

type Widget interface {
	Render(image.Point) Block
	Size() image.Point
	SetSize(image.Point)
	Origin() image.Point
	SetOrigin(image.Point)
}
