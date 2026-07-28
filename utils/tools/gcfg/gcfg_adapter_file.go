package gcfg

import (
	"context"
	"fmt"
	"gofly/utils/tools/gconv"
	"gofly/utils/tools/gfile"
	"gofly/utils/tools/gjson"
	"gofly/utils/tools/instance"
	"path/filepath"
)

// AdapterFile implements interface Adapter using file.
type AdapterFile struct {
	defaultFileName string // Default configuration file name or file path.
	defaultFilePath string // Default configuration file directory.
}

// SetFileName sets the default configuration file name.
func (a *AdapterFile) SetFileName(fileName string) {
	a.defaultFileName = fileName
}

// SetFilePath sets the default configuration file path.
func (a *AdapterFile) SetFilePath(filePath string) {
	a.defaultFilePath = filePath
}

// NewAdapterFile returns a new configuration management object.
func NewAdapterFile(fileNameOrPath ...string) (*AdapterFile, error) {
	var (
		usedefaultFileName = DefaultConfigFileName
		usedefaultFilePath = DefaultConfigFilePath
	)
	if len(fileNameOrPath) > 0 {
		usedefaultFileName = fileNameOrPath[0]
	}
	if len(fileNameOrPath) > 1 {
		usedefaultFilePath = fileNameOrPath[1]
	}
	config := &AdapterFile{
		defaultFileName: usedefaultFileName,
		defaultFilePath: usedefaultFilePath,
	}
	return config, nil
}

// Get retrieves and returns value by specified `pattern`.
// It returns all values of the current JSON object if `pattern` is given empty or string ".".
// It returns nil if no value found by `pattern`.
//
// We can also access slice item by its index number in `pattern` like:
// "list.10", "array.0.name", "array.0.1.id".
//
// It returns a default value specified by `def` if value for `pattern` is not found.
func (a *AdapterFile) Get(ctx context.Context, pattern string) (value interface{}, err error) {
	j, err := a.getJson()
	if err != nil {
		return nil, err
	}
	if j != nil {
		return j.Get(pattern).Val(), nil
	}
	return nil, nil
}

// Data retrieves and returns all configuration data as map type.
func (a *AdapterFile) Data(ctx context.Context) (data map[string]interface{}, err error) {
	j, err := a.getJson()
	if err != nil {
		return nil, err
	}
	if j != nil {
		return gconv.Map(j.Var()), nil
	}
	return nil, nil
}

// getJson returns a *gjson.Json object for the specified `file` content.
// It would print error if file reading fails. It returns nil if any error occurs.
func (a *AdapterFile) getJson() (configJson *gjson.Json, err error) {
	var (
		usedFileName = a.defaultFileName
		usedFilePath = a.defaultFilePath
		realPath     string
		fileExt      string
	)
	//  Iterate supported file types to find the existing config file
	for _, ext := range supportedFileTypes {
		filePath := filepath.Join(usedFilePath, usedFileName+"."+ext)
		realPath = gfile.RealPath(filePath)
		if realPath != "" {
			fileExt = ext
			break // Exit loop once the first valid file is found
		}
	}

	if realPath == "" {
		err = fmt.Errorf("no config file found, supported types: %v", supportedFileTypes)
		return
	}
	val := instance.GetOrSetFuncLock(realPath, func() interface{} {
		contentByte := gfile.GetBytes(realPath)
		switch fileExt {
		case "yaml", "yml":
			configJson, err = gjson.LoadYaml(contentByte)
		case "toml":
			configJson, err = gjson.LoadToml(contentByte)
		case "xml":
			configJson, err = gjson.LoadXml(contentByte)
		case "json":
			configJson, err = gjson.LoadJson(contentByte)
		default:
			err = fmt.Errorf("unsupported file extension: %s", fileExt)
		}
		if err != nil {
			panic(err)
		}
		return configJson
	})
	configJson, ok := val.(*gjson.Json)
	if !ok || configJson == nil {
		return nil, fmt.Errorf("load config failed, type convert error")
	}
	return configJson, err
}

// ClearCache clears config cache for specified config file hot-reload
// realPath is the absolute file path retrieved by gfile.RealPath()
func (a *AdapterFile) ClearCache() error {
	var (
		usedFileName = a.defaultFileName
		usedFilePath = a.defaultFilePath
		realPath     string
	)
	//  Iterate supported file types to find the existing config file
	for _, ext := range supportedFileTypes {
		filePath := filepath.Join(usedFilePath, usedFileName+"."+ext)
		realPath = gfile.RealPath(filePath)
		if realPath != "" {
			break // Exit loop once the first valid file is found
		}
	}

	if realPath == "" {
		return fmt.Errorf("no config file found, supported types: %v", supportedFileTypes)
	}
	instance.ClearOne(realPath)
	return nil
}

// ClearAllCache clears all config caches
// realPath parameter is reserved (unused here), original comment description removed for accuracy
func (a *AdapterFile) ClearAllCache() {
	instance.Clear()
}
