package srvfiber

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestFiberConfig(t *testing.T) {
	cfg := &PlatformConfig{
		BindAddress: "wantBindAddress",
	}
	assert.Implements(t, (*ServerFiberConfig)(nil), cfg)
	assert.Equal(t, cfg.BindAddress, cfg.GetBindAddress())
}
