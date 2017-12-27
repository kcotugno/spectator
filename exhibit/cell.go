package exhibit

type Position struct {
	X int
	Y int
}

type Cell struct {
	Pos   Position
	Value rune
	Attrs Attributes
}
