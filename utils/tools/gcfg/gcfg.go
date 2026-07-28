// Package gcfg provides reading, caching and managing for configuration.
package gcfg

import (
	"context"
	"gofly/utils/tools/gvar"
	"gofly/utils/tools/intlog"
)

// Config is the configuration management object.
type Config struct {
	adapter Adapter
}

var (
	supportedFileTypes = []string{"toml", "yaml", "yml", "json", "xml"}
)

const (
	// DefaultConfigFileName is the default configuration file name.
	DefaultConfigFileName = "config"
	//Project configuration file directory
	DefaultConfigFilePath = "resource/config"
)

// NewWithAdapter creates and returns a Config object with given adapter.
func NewWithAdapter(adapter Adapter) *Config {
	return &Config{
		adapter: adapter,
	}
}

// Instance returns an instance of Config with default settings.
// The parameter `name` is the name for the instance. But very note that, if the file "name.toml"
// exists in the configuration directory, it then sets it as the default configuration file. The
// toml file type is the default configuration file type.
func Instance(name ...string) *Config {
	var instanceName = DefaultConfigFileName
	if len(name) > 0 && name[0] != "" {
		instanceName = name[0]
	}
	adapterFile, err := NewAdapterFile()
	if err != nil {
		intlog.Errorf(context.Background(), `%+v`, err)
		return nil
	}
	if instanceName != DefaultConfigFileName {
		adapterFile.SetFileName(instanceName)
	}
	if len(name) > 1 && name[1] != "" {
		adapterFile.SetFilePath(name[1])
	}
	return NewWithAdapter(adapterFile)
}

func (c *Config) Get(ctx context.Context, pattern string, def ...interface{}) (*gvar.Var, error) {
	var (
		err   error
		value interface{}
	)
	value, err = c.adapter.Get(ctx, pattern)
	if err != nil {
		return nil, err
	}
	if value == nil {
		if len(def) > 0 {
			return gvar.New(def[0]), nil
		}
		return nil, nil
	}
	return gvar.New(value), nil
}

// Data retrieves and returns all configuration data as map type.
func (c *Config) Data(ctx context.Context) (data map[string]interface{}, err error) {
	return c.adapter.Data(ctx)
}

// Clear the specified configuration cache
func (c *Config) ClearCache() error {
	return c.adapter.ClearCache()
}

// ClearAllCache clears all config caches
func (c *Config) ClearAllCache() {
	c.adapter.ClearAllCache()
}
