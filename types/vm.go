package types

type VMType int

const (
	MoveVM VMType = iota
	WasmVM
	EVM
)
