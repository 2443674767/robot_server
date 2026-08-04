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
	RegisterDogDriver("default", pythonDog)
	RegisterDogDriver("m20", pythonDog)
	RegisterDogDriver("yunshenchu_m20", pythonDog)
	RegisterDogDriver("云深处m20", pythonDog)
	RegisterPTZDriver("default", NewDefaultPTZDriver())
}
