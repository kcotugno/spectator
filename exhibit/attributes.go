package exhibit

const (
	Normal = iota
	Bold
	_
	_
	Underline
	SlowBlink
)

const (
	FGBlack = ForegroundColor(iota + 30)
	FGRed
	FGGreen
	FGYellow
	FGBlue
	FGMagenta
	FGCyan
	FGWhite
)

const (
	BGBlack = BackgroundColor(iota + 40)
	BGRed
	BGGreen
	BGYellow
	BGBlue
	BGMagenta
	BGCyan
	BGWhite
)

type Attributes struct {
	ForegroundColor ForegroundColor
	BackgroundColor BackgroundColor
	Bold            bool
	Italics         bool
	Blink           bool
	Underline       bool
}

type ForegroundColor int
type BackgroundColor int
