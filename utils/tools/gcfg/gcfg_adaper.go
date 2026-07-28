package gcfg

import "context"

// Adapter is the interface for configuration retrieving.
type Adapter interface {

	// Get retrieves and returns value by specified `pattern` in current resource.
	// Pattern like:
	// "x.y.z" for map item.
	// "x.0.y" for slice item.
	Get(ctx context.Context, pattern string) (value interface{}, err error)

	// Data retrieves and returns all configuration data in current resource as map.
	// Note that this function may lead lots of memory usage if configuration data is too large,
	// you can implement this function if necessary.
	Data(ctx context.Context) (data map[string]interface{}, err error)
	// Clear the specified configuration cache
	ClearCache() error
	// ClearAllCache clears all config caches
	ClearAllCache()
}
