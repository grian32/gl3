package sema

import "gl3/util"

type Diagnostic struct {
	Message  string
	Position *util.Position
}
