package exhibit

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
