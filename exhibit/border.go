package exhibit

import (
	"image"
)

const (
	Thick = Style(iota)
	Thin
	ThickBroken
	ThinBroken
	Double
)

const (
	Vertical = Box(iota)
	Horizontal
	TopRight
	TopLeft
	BottomRight
	BottomLeft
	VerticalRight
	VerticalLeft
	HorizontalUp
	HorizontalDown
	Intersect
)

type Border struct {
	Style
	Box
	Attributes
	Visible bool
}

type Style int
type Box int

var thick = []rune{'┃', '━', '┓', '┏', '┛', '┗', '┣', '┫', '┻', '┳', '╋'}

var thin = []rune{'│', '─', '┐', '┌', '┘', '└', '├', '┤', '┴', '┬', '┼'}

var thickBroken = []rune{'┇', '┅', '┓', '┏', '┛', '┗', '┣', '┫', '┻', '┳', '╋'}

var thinBroken = []rune{'┆', '┄', '┐', '┌', '┘', '└', '├', '┤', '┴', '┬', '┼'}

var double = []rune{'║', '═', '╗', '╔', '╝', '╚', '╠', '╣', '╩', '╦', '╬'}

func BorderRune(c Box, s Style) rune {
	switch s {
	case Thick:
		return thick[c]
	case Thin:
		return thin[c]
	case ThickBroken:
		return thickBroken[c]
	case ThinBroken:
		return thinBroken[c]
	case Double:
		return double[c]
	default:
		return ' '
	}
}

func (b Border) Cell(p image.Point, r image.Rectangle) (Cell, bool) {
	c := Cell{}

	if !b.Visible {
		return c, false
	}

	c.Attrs = b.Attributes

	if p.X != r.Min.X && p.X != r.Max.X-1 &&
		p.Y != r.Min.Y && p.Y != r.Max.Y-1 {
		return c, false
	}

	if p.X == r.Min.X && p.Y == r.Min.Y {
		c.Value = BorderRune(TopLeft, b.Style)
	} else if p.X == r.Max.X-1 && p.Y == r.Min.Y {
		c.Value = BorderRune(TopRight, b.Style)
	} else if p.X == r.Min.X && p.Y == r.Max.Y-1 {
		c.Value = BorderRune(BottomLeft, b.Style)
	} else if p.X == r.Max.X-1 && p.Y == r.Max.Y-1 {
		c.Value = BorderRune(BottomRight, b.Style)
	} else if p.X == r.Min.X || p.X == r.Max.X-1 {
		c.Value = BorderRune(Vertical, b.Style)
	} else if p.Y == r.Min.Y || p.Y == r.Max.Y-1 {
		c.Value = BorderRune(Horizontal, b.Style)
	}

	return c, true
}
