package session

import "context"

// State is a recursive type that represents
// some node in a state machine graph.  It
// takes a context and returns the next state
// to execute.
type State func(context.Context, *Session) (State, error)
