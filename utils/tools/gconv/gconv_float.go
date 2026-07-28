package gconv

// Float32 converts `any` to float32.
func Float32(any any) float32 {
	v, _ := defaultConverter.Float32(any)
	return v
}

// Float64 converts `any` to float64.
func Float64(any any) float64 {
	v, _ := defaultConverter.Float64(any)
	return v
}
