package hir

type Symbol interface {
	symbol()
}

type StructID uint32

func (StructID) symbol() {}

type FunctionID uint32

func (FunctionID) symbol() {}

type GlobalID uint32

func (GlobalID) symbol() {}

type LocalID uint32

func (LocalID) symbol() {}
