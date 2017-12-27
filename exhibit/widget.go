package exhibit

type Border struct {
	Top    bool
	Bottom bool
	Left   bool
	Right  bool
}
type Constraints struct {
	Top    bool
	Bottom bool
	Left   bool
	Right  bool
}

type Widget interface {
	Render() [][]Cell
}
