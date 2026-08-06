package control

func buildFrame(device byte, command byte, value byte, duration uint16) []byte {
	payload := []byte{
		0xAA,
		0x55,
		device,
		command,
		value,
		byte(duration >> 8),
		byte(duration),
	}
	var checksum byte
	for _, b := range payload {
		checksum ^= b
	}
	return append(payload, checksum)
}

func init() {
	pythonDog := NewPythonDogDriver()
	pythonPTZ := NewPythonPTZDriver()
	libra4PTZ := NewLibra4PTZDriver()
	RegisterDogDriver("default", pythonDog)
	RegisterDogDriver("m20", pythonDog)
	RegisterDogDriver("yunshenchu_m20", pythonDog)
	RegisterDogDriver("云深处m20", pythonDog)
	RegisterPTZDriver("default", pythonPTZ)
	RegisterPTZDriver("hy", pythonPTZ)
	RegisterPTZDriver("hy-dz230f", pythonPTZ)
	RegisterPTZDriver("hy_dz230f", pythonPTZ)
	RegisterPTZDriver("汇云", pythonPTZ)
	RegisterPTZDriver("libra4", libra4PTZ)
	RegisterPTZDriver("ptz_libra4", libra4PTZ)
	RegisterPTZDriver("LIBRA4", libra4PTZ)
}
