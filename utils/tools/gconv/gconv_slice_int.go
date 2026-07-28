package gconv

// SliceInt is alias of Ints.
func SliceInt(any any) []int {
	return Ints(any)
}

// SliceInt32 is alias of Int32s.
func SliceInt32(any any) []int32 {
	return Int32s(any)
}

// SliceInt64 is alias of Int64s.
func SliceInt64(any any) []int64 {
	return Int64s(any)
}

// Ints converts `any` to []int.
func Ints(any any) []int {
	result, _ := defaultConverter.SliceInt(any, SliceOption{
		ContinueOnError: true,
	})
	return result
}

// Int32s converts `any` to []int32.
func Int32s(any any) []int32 {
	result, _ := defaultConverter.SliceInt32(any, SliceOption{
		ContinueOnError: true,
	})
	return result
}

// Int64s converts `any` to []int64.
func Int64s(any any) []int64 {
	result, _ := defaultConverter.SliceInt64(any, SliceOption{
		ContinueOnError: true,
	})
	return result
}
