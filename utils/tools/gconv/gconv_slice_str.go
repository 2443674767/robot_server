package gconv

// SliceStr is alias of Strings.
func SliceStr(any interface{}) []string {
	return Strings(any)
}

// Strings converts `any` to []string.
func Strings(any interface{}) []string {
	result, _ := defaultConverter.SliceStr(any, SliceOption{
		ContinueOnError: true,
	})
	return result
}
