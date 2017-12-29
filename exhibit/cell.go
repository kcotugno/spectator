package exhibit

import (
	"image"
)

type Cell struct {
	Value rune
	Point image.Point
	Attrs Attributes
}
