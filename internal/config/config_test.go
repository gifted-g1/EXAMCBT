package config

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestProductionRejectsWeakSecrets(t *testing.T) {
	t.Setenv("APP_ENV", "production")
	t.Setenv("MODE", "standalone")
	t.Setenv("JWT_SECRET", "change-me-in-production")
	_, err := Load()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "JWT_SECRET")

	t.Setenv("JWT_SECRET", "a3f1c9e0b7d24e6f8a1b2c3d4e5f60718293a4b5c6d7e8f9")
	t.Setenv("BCRYPT_COST", "10")
	_, err = Load()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "BCRYPT_COST")
}

func TestCIDRParsingAndDistributedRequirements(t *testing.T) {
	t.Setenv("LAN_CIDRS", "192.168.1.0/24, 10.0.0.5")
	t.Setenv("MODE", "distributed")
	_, err := Load()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "DB_DSN")

	t.Setenv("MODE", "standalone")
	c, err := Load()
	require.NoError(t, err)
	require.Len(t, c.LANCIDRs, 2)
	assert.Equal(t, "10.0.0.5/32", c.LANCIDRs[1].String())

	t.Setenv("LAN_CIDRS", "192.168.1.0/33")
	_, err = Load()
	assert.Error(t, err)
	t.Setenv("LAN_CIDRS", "")
	t.Setenv("TRUSTED_PROXIES", "nginx")
	_, err = Load()
	assert.Error(t, err)
}
