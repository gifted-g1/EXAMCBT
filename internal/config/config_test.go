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

func TestHostingPlatformSettings(t *testing.T) {
	t.Setenv("MODE", "standalone")
	t.Setenv("PORT", "10000")
	c, err := Load()
	require.NoError(t, err)
	assert.Equal(t, ":10000", c.HTTPAddr, "PORT is honoured when HTTP_ADDR is unset")
	assert.False(t, c.OpenAccess(), "LAN lock is the default")
	assert.Empty(t, c.TrustedProxies)

	t.Setenv("HTTP_ADDR", ":9999")
	t.Setenv("ACCESS_MODE", "OPEN")
	t.Setenv("PUBLIC_URL", "https://examcbt.onrender.com/")
	c, err = Load()
	require.NoError(t, err)
	assert.Equal(t, ":9999", c.HTTPAddr, "explicit HTTP_ADDR wins over PORT")
	assert.True(t, c.OpenAccess())
	assert.Equal(t, "https://examcbt.onrender.com", c.PublicURL)
	assert.Contains(t, c.TrustedProxies, "10.0.0.0/8", "open mode trusts the platform load balancer")

	t.Setenv("ACCESS_MODE", "everyone")
	_, err = Load()
	assert.ErrorContains(t, err, "ACCESS_MODE")
}
