package gjson

import (
	"gofly/utils/tools/gvar"
)

// Interface returns the json value.
func (j *Json) Interface() interface{} {
	if j == nil {
		return nil
	}
	j.mu.RLock()
	defer j.mu.RUnlock()
	if j.p == nil {
		return nil
	}
	return *(j.p)
}

// Var returns the json value as *gvar.Var.
func (j *Json) Var() interface{} {
	return j.Interface()
}

// IsNil checks whether the value pointed by `j` is nil.
func (j *Json) IsNil() bool {
	if j == nil {
		return true
	}
	j.mu.RLock()
	defer j.mu.RUnlock()
	return j.p == nil || *(j.p) == nil
}

// Get retrieves and returns value by specified `pattern`.
// It returns all values of current Json object if `pattern` is given ".".
// It returns nil if no value found by `pattern`.
//
// We can also access slice item by its index number in `pattern` like:
// "list.10", "array.0.name", "array.0.1.id".
//
// It returns a default value specified by `def` if value for `pattern` is not found.
func (j *Json) Get(pattern string, def ...interface{}) *gvar.Var {
	if j == nil {
		return nil
	}
	j.mu.RLock()
	defer j.mu.RUnlock()

	// It returns nil if pattern is empty.
	if pattern == "" {
		return nil
	}

	result := j.getPointerByPattern(pattern)
	if result != nil {
		return gvar.New(*result)
	}
	if len(def) > 0 {
		return gvar.New(def[0])
	}
	return nil
}
