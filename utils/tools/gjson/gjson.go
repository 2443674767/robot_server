// Package gjson provides convenient API for JSON/XML/INI/YAML/TOML data handling.
package gjson

import (
	"gofly/utils/tools/gstr"
	"gofly/utils/tools/rwmutex"
	"strconv"
	"strings"
)

type ContentType string

const (
	ContentTypeJson  ContentType = `json`
	ContentTypeXml   ContentType = `xml`
	ContentTypeYaml  ContentType = `yaml`
	ContentTypeYml   ContentType = `yml`
	ContentTypeToml  ContentType = `toml`
	defaultSplitChar             = '.' // Separator char for hierarchical data access.
)

// iVal is the interface for underlying interface{} retrieving.
type iVal interface {
	Val() interface{}
}

// Options for Json object creating/loading.
type Options struct {
	Safe      bool        // Mark this object is for in concurrent-safe usage. This is especially for Json object creating.
	Tags      string      // Custom priority tags for decoding, eg: "json,yaml,MyTag". This is especially for struct parsing into Json object.
	Type      ContentType // Type specifies the data content type, eg: json, xml, yaml, toml, ini.
	StrNumber bool        // StrNumber causes the Decoder to unmarshal a number into an interface{} as a string instead of as a float64.
}

// Json is the customized JSON struct.
type Json struct {
	mu rwmutex.RWMutex
	p  *interface{} // Pointer for hierarchical data access, it's the root of data in default.
	c  byte         // Char separator('.' in default).
	vc bool         // Violence Check(false in default), which is used to access data when the hierarchical data key contains separator char.
}

// getPointerByPattern returns a pointer to the value by specified `pattern`.
func (j *Json) getPointerByPattern(pattern string) *interface{} {
	if j.p == nil {
		return nil
	}
	if j.vc {
		return j.getPointerByPatternWithViolenceCheck(pattern)
	} else {
		return j.getPointerByPatternWithoutViolenceCheck(pattern)
	}
}

// getPointerByPatternWithViolenceCheck returns a pointer to the value of specified `pattern` with violence check.
func (j *Json) getPointerByPatternWithViolenceCheck(pattern string) *interface{} {
	if !j.vc {
		return j.getPointerByPatternWithoutViolenceCheck(pattern)
	}

	// It returns nil if pattern is empty.
	if pattern == "" {
		return nil
	}
	// It returns all if pattern is ".".
	if pattern == "." {
		return j.p
	}

	var (
		index   = len(pattern)
		start   = 0
		length  = 0
		pointer = j.p
	)
	if index == 0 {
		return pointer
	}
	for {
		if r := j.checkPatternByPointer(pattern[start:index], pointer); r != nil {
			if length += index - start; start > 0 {
				length += 1
			}
			start = index + 1
			index = len(pattern)
			if length == len(pattern) {
				return r
			} else {
				pointer = r
			}
		} else {
			// Get the position for next separator char.
			index = strings.LastIndexByte(pattern[start:index], j.c)
			if index != -1 && length > 0 {
				index += length + 1
			}
		}
		if start >= index {
			break
		}
	}
	return nil
}

// getPointerByPatternWithoutViolenceCheck returns a pointer to the value of specified `pattern`, with no violence check.
func (j *Json) getPointerByPatternWithoutViolenceCheck(pattern string) *interface{} {
	if j.vc {
		return j.getPointerByPatternWithViolenceCheck(pattern)
	}
	// It returns nil if pattern is empty.
	if pattern == "" {
		return nil
	}
	// It returns all if pattern is ".".
	if pattern == "." {
		return j.p
	}

	pointer := j.p
	if len(pattern) == 0 {
		return pointer
	}
	array := strings.Split(pattern, string(j.c))
	for k, v := range array {
		if r := j.checkPatternByPointer(v, pointer); r != nil {
			if k == len(array)-1 {
				return r
			} else {
				pointer = r
			}
		} else {
			break
		}
	}
	return nil
}

// checkPatternByPointer checks whether there's value by `key` in specified `pointer`.
// It returns a pointer to the value.
func (j *Json) checkPatternByPointer(key string, pointer *interface{}) *interface{} {
	switch (*pointer).(type) {
	case map[string]interface{}:
		if v, ok := (*pointer).(map[string]interface{})[key]; ok {
			return &v
		}
	case []interface{}:
		if gstr.IsNumeric(key) {
			n, err := strconv.Atoi(key)
			if err == nil && len((*pointer).([]interface{})) > n {
				return &(*pointer).([]interface{})[n]
			}
		}
	}
	return nil
}
