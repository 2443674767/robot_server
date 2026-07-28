// Package gyaml provides accessing and converting for YAML content.
package gyaml

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"gopkg.in/yaml.v3"

	"gofly/utils/tools/gconv"
)

// Encode encodes `value` to an YAML format content as bytes.
func Encode(value interface{}) (out []byte, err error) {
	out, err = yaml.Marshal(value)
	return
}

// EncodeIndent encodes `value` to an YAML format content with indent as bytes.
func EncodeIndent(value interface{}, indent string) (out []byte, err error) {
	out, err = Encode(value)
	if err != nil {
		return
	}
	if indent != "" {
		var (
			buffer = bytes.NewBuffer(nil)
			array  = strings.Split(strings.TrimSpace(string(out)), "\n")
		)
		for _, v := range array {
			buffer.WriteString(indent)
			buffer.WriteString(v)
			buffer.WriteString("\n")
		}
		out = buffer.Bytes()
	}
	return
}

// Decode parses `content` into and returns as map.
func Decode(content []byte) (map[string]interface{}, error) {
	var (
		result map[string]interface{}
		err    error
	)
	if err = yaml.Unmarshal(content, &result); err != nil {
		return nil, err
	}
	return gconv.MapDeep(result), nil
}

// DecodeTo parses `content` into `result`.
func DecodeTo(value []byte, result interface{}) (err error) {
	err = yaml.Unmarshal(value, result)
	return
}

// ToJson converts `content` to JSON format content.
func ToJson(content []byte) (out []byte, err error) {
	var (
		result interface{}
	)
	if result, err = Decode(content); err != nil {
		return nil, err
	} else {
		return json.Marshal(result)
	}
}

// SetYamlValue modifies the values of YAML fields at the level, while preserving the comment format.
// 层级修改yaml字段值，保留注释格式
// path：层级路径，如 []string{"iprule","open"}  []string{"cc","winSec"}或SetYamlValue("websec.yaml", []string{"open"}, true)
// newValue：支持 bool/int/string
func SetYamlValue(filePath string, path []string, newValue any) error {
	// 读取文件
	content, err := os.ReadFile(filePath)
	if err != nil {
		return fmt.Errorf("读取文件失败: %w", err)
	}

	var root yaml.Node
	if err := yaml.Unmarshal(content, &root); err != nil {
		return fmt.Errorf("解析yaml失败: %w", err)
	}

	// 根节点校验
	if root.Kind != yaml.DocumentNode || len(root.Content) == 0 {
		return fmt.Errorf("yaml格式错误，无根映射")
	}

	currentNode := root.Content[0]

	// 逐层遍历查找key
	for level, key := range path {
		found := false
		for i := 0; i < len(currentNode.Content); i += 2 {
			keyNode := currentNode.Content[i]
			valNode := currentNode.Content[i+1]
			if keyNode.Value == key {
				// 最后一层：修改值
				if level == len(path)-1 {
					switch v := newValue.(type) {
					case bool:
						valNode.Tag = "!!bool"
						valNode.Value = fmt.Sprintf("%t", v)
					case int:
						valNode.Tag = "!!int"
						valNode.Value = fmt.Sprintf("%d", v)
					case string:
						valNode.Tag = "!!str"
						valNode.Value = v
					}
					found = true
					break
				}
				// 非最后一层，进入子map
				currentNode = valNode
				found = true
				break
			}
		}
		if !found {
			return fmt.Errorf("未找到配置路径: %v", path)
		}
	}

	var buf strings.Builder
	enc := yaml.NewEncoder(&buf)
	enc.SetIndent(2) // 强制输出2空格缩进，和原文件统一
	defer enc.Close()

	if err := enc.Encode(&root); err != nil {
		return fmt.Errorf("编码yaml失败: %w", err)
	}
	out := []byte(buf.String())
	return os.WriteFile(filePath, out, 0644)
}
