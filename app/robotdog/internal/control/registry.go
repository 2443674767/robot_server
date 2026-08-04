package control

import (
	"fmt"
	"strings"
	"sync"
)

var (
	dogMu      sync.RWMutex
	dogDrivers = map[string]DogController{}

	ptzMu      sync.RWMutex
	ptzDrivers = map[string]PTZController{}
)

func RegisterDogDriver(model string, driver DogController) {
	dogMu.Lock()
	defer dogMu.Unlock()
	dogDrivers[normalizeKey(model)] = driver
}

func RegisterPTZDriver(model string, driver PTZController) {
	ptzMu.Lock()
	defer ptzMu.Unlock()
	ptzDrivers[normalizeKey(model)] = driver
}

func DogDriver(model string) (DogController, string, error) {
	dogMu.RLock()
	defer dogMu.RUnlock()
	key := normalizeKey(model)
	if driver, ok := dogDrivers[key]; ok {
		return driver, key, nil
	}
	if driver, ok := dogDrivers["default"]; ok {
		return driver, "default", nil
	}
	return nil, "", fmt.Errorf("机械狗驱动未注册: %s", model)
}

func PTZDriver(model string) (PTZController, string, error) {
	ptzMu.RLock()
	defer ptzMu.RUnlock()
	key := normalizeKey(model)
	if driver, ok := ptzDrivers[key]; ok {
		return driver, key, nil
	}
	if driver, ok := ptzDrivers["default"]; ok {
		return driver, "default", nil
	}
	return nil, "", fmt.Errorf("云台驱动未注册: %s", model)
}

func normalizeKey(v string) string {
	v = strings.TrimSpace(strings.ToLower(v))
	if v == "" {
		return "default"
	}
	return v
}
