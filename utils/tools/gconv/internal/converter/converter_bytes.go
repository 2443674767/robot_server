package converter

import (
	"math"
	"reflect"

	"gofly/utils/tools/empty"
	"gofly/utils/tools/gconv/internal/localinterface"
	"gofly/utils/tools/json"
	"gofly/utils/tools/reflection"

	"gofly/utils/tools/gbinary"
)

// Bytes converts `any` to []byte.
func (c *Converter) Bytes(any any) ([]byte, error) {
	if empty.IsNil(any) {
		return nil, nil
	}
	switch value := any.(type) {
	case string:
		return []byte(value), nil

	case []byte:
		return value, nil

	default:
		if f, ok := value.(localinterface.IBytes); ok {
			return f.Bytes(), nil
		}
		originValueAndKind := reflection.OriginValueAndKind(any)
		switch originValueAndKind.OriginKind {
		case reflect.Map:
			bytes, err := json.Marshal(any)
			if err != nil {
				return nil, err
			}
			return bytes, nil

		case reflect.Array, reflect.Slice:
			var (
				ok    = true
				bytes = make([]byte, originValueAndKind.OriginValue.Len())
			)
			for i := range bytes {
				int32Value, err := c.Int32(originValueAndKind.OriginValue.Index(i).Interface())
				if err != nil {
					return nil, err
				}
				if int32Value < 0 || int32Value > math.MaxUint8 {
					ok = false
					break
				}
				bytes[i] = byte(int32Value)
			}
			if ok {
				return bytes, nil
			}
		default:
		}
		return gbinary.Encode(any), nil
	}
}
