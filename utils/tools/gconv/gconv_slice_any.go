package gconv

// SliceAny is alias of Interfaces.
func SliceAny(any interface{}) []interface{} {
	return Interfaces(any)
}

// Interfaces converts `any` to []interface{}.
func Interfaces(any interface{}) []interface{} {
	result, _ := defaultConverter.SliceAny(any, SliceOption{
		ContinueOnError: true,
	})
	return result
}

// IsSlice checks whether a value is a slice or an array.
func IsSlice(any interface{}) bool {
	return defaultConverter.IsSlice(any)
}
