package main

import (
	"git.kevincotugno.com/kcotugno/spectator/exhibit"
)

type ListEntry struct {
	Value string
	Attrs exhibit.Attributes
}

func (e ListEntry) String() string {
	return e.Value
}

func (e ListEntry) Attributes() exhibit.Attributes {
	return e.Attrs
}
