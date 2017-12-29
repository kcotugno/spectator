package exhibit

type Border struct {
	Top    bool
	Bottom bool
	Left   bool
	Right  bool
}

type Widget interface {
	Render() [][]Cell
	Constraints() Constraints
	SetConstraints(Constraints)
	Size() Size
	SetSize(size Size)
	Attributes() Attributes
	SetAttributes(attrs Attributes)
}
