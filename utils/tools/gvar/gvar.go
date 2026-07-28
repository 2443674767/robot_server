// Package gvar provides an universal variable type, like runtime generics.
package gvar

import (
	"sync/atomic"
)

// Var is an universal variable type implementer.
type Var struct {
	value interface{} // Underlying value.
	safe  bool        // Concurrent safe or not.
}

// Interface is a struct for concurrent-safe operation for type interface{}.
type Interface struct {
	value atomic.Value
}

// NewInterface creates and returns a concurrent-safe object for interface{} type,
// with given initial value `value`.
func NewInterface(value ...interface{}) *Interface {
	t := &Interface{}
	if len(value) > 0 && value[0] != nil {
		t.value.Store(value[0])
	}
	return t
}

// New creates and returns a new Var with given `value`.
// The optional parameter `safe` specifies whether Var is used in concurrent-safety,
// which is false in default.
func New(value interface{}, safe ...bool) *Var {
	if len(safe) > 0 && safe[0] {
		return &Var{
			value: NewInterface(value),
			safe:  true,
		}
	}
	return &Var{
		value: value,
	}
}

// Set atomically stores `value` into t.value and returns the previous value of t.value.
// Note: The parameter `value` cannot be nil.
func (v *Interface) Set(value interface{}) (old interface{}) {
	old = v.Val()
	v.value.Store(value)
	return
}

// Val atomically loads and returns t.value.
func (v *Interface) Val() interface{} {
	return v.value.Load()
}
