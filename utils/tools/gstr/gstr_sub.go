package gstr

// 将str从start索引位置（索引从0开始）进行字符串截取length，支持中文，类似PHP的substr函数
// SubStr returns a portion of string `str` specified by the `start` and `length` parameters.
// The parameter `length` is optional, it uses the length of `str` in default.
//
// Example:
// SubStr("123456", 1, 2) -> "23"
func SubStr(str string, start int, length ...int) (substr string) {
	strLength := len(str)
	if start < 0 {
		if -start > strLength {
			start = 0
		} else {
			start = strLength + start
		}
	} else if start > strLength {
		return ""
	}
	realLength := 0
	if len(length) > 0 {
		realLength = length[0]
		if realLength < 0 {
			if -realLength > strLength-start {
				realLength = 0
			} else {
				realLength = strLength - start + realLength
			}
		} else if realLength > strLength-start {
			realLength = strLength - start
		}
	} else {
		realLength = strLength - start
	}

	if realLength == strLength {
		return str
	} else {
		end := start + realLength
		return str[start:end]
	}
}
