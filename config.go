package srvfiber

// ServerFiberConfig defines the configuration contract to be used on the FiberServer.
type ServerFiberConfig interface {
	GetBindAddress() string
}

// PlatformConfig is the default implementation of the ServerFiberConfig using the github.com/setare/go-config library.
type PlatformConfig struct {
	BindAddress string `config:"bind_address"`
}

func (config *PlatformConfig) GetBindAddress() string {
	return config.BindAddress
}
