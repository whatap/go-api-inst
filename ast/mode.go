package ast

// Mode determines whether the engine injects or removes instrumentation.
type Mode int

const (
	ModeInject Mode = iota
	ModeRemove
)
