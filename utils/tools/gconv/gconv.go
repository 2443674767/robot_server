// Package gconv implements powerful and convenient converting functionality for any types of variables.
//
// This package should keep much fewer dependencies with other packages.
package gconv

import (
	"reflect"
	"time"

	"gofly/utils/tools/gconv/internal/converter"
	"gofly/utils/tools/gconv/internal/localinterface"
	"gofly/utils/tools/gconv/internal/structcache"
)

// Converter is the manager for type converting.
type Converter interface {
	ConverterForRegister
	ConverterForInt
	ConverterForUint
	ConverterForTime
	ConverterForFloat
	ConverterForSlice
	ConverterForBasic
}

// ConverterForBasic is the basic converting interface.
type ConverterForBasic interface {
	String(any any) (string, error)
	Bool(any any) (bool, error)
	Rune(any any) (rune, error)
}

// ConverterForTime is the converting interface for time.
type ConverterForTime interface {
	Time(v any, format ...string) (time.Time, error)
	Duration(v any) (time.Duration, error)
}

// ConverterForInt is the converting interface for integer.
type ConverterForInt interface {
	Int(v any) (int, error)
	Int8(v any) (int8, error)
	Int16(v any) (int16, error)
	Int32(v any) (int32, error)
	Int64(v any) (int64, error)
}

// ConverterForUint is the converting interface for unsigned integer.
type ConverterForUint interface {
	Uint(v any) (uint, error)
	Uint8(v any) (uint8, error)
	Uint16(v any) (uint16, error)
	Uint32(v any) (uint32, error)
	Uint64(v any) (uint64, error)
}

// ConverterForFloat is the converting interface for float.
type ConverterForFloat interface {
	Float32(v any) (float32, error)
	Float64(v any) (float64, error)
}

// ConverterForSlice is the converting interface for slice.
type ConverterForSlice interface {
	Bytes(v any) ([]byte, error)
	Runes(v any) ([]rune, error)
	SliceAny(v any, option ...SliceOption) ([]any, error)
	SliceFloat32(v any, option ...SliceOption) ([]float32, error)
	SliceFloat64(v any, option ...SliceOption) ([]float64, error)
	SliceInt(v any, option ...SliceOption) ([]int, error)
	SliceInt32(v any, option ...SliceOption) ([]int32, error)
	SliceInt64(v any, option ...SliceOption) ([]int64, error)
	SliceUint(v any, option ...SliceOption) ([]uint, error)
	SliceUint32(v any, option ...SliceOption) ([]uint32, error)
	SliceUint64(v any, option ...SliceOption) ([]uint64, error)
	SliceStr(v any, option ...SliceOption) ([]string, error)
}

// ConverterForRegister is the converting interface for custom converter registration.
type ConverterForRegister interface {
	RegisterTypeConverterFunc(f any) error
	RegisterAnyConverterFunc(f AnyConvertFunc, types ...reflect.Type)
}

type (
	// AnyConvertFunc is the function type for converting any to specified type.
	AnyConvertFunc = structcache.AnyConvertFunc

	// MapOption specifies the option for map converting.
	MapOption = converter.MapOption

	// SliceOption is the option for Slice type converting.
	SliceOption = converter.SliceOption
)

// IUnmarshalValue is the interface for custom defined types customizing value assignment.
// Note that only pointer can implement interface IUnmarshalValue.
type IUnmarshalValue = localinterface.IUnmarshalValue

var (
	// defaultConverter is the default management object converting.
	defaultConverter = converter.NewConverter()
)

// RegisterConverter registers custom converter.
// Deprecated: use RegisterTypeConverterFunc instead for clear
func RegisterConverter(fn any) (err error) {
	return RegisterTypeConverterFunc(fn)
}

// RegisterTypeConverterFunc registers custom converter.
func RegisterTypeConverterFunc(fn any) (err error) {
	return defaultConverter.RegisterTypeConverterFunc(fn)
}

// RegisterAnyConverterFunc registers custom type converting function for specified type.
func RegisterAnyConverterFunc(f AnyConvertFunc, types ...reflect.Type) {
	defaultConverter.RegisterAnyConverterFunc(f, types...)
}
