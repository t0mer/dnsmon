package geoip_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/t0mer/dnsmon/internal/geoip"
)

func TestOpen_MissingFile(t *testing.T) {
	db, err := geoip.Open("/nonexistent/path/GeoLite2-City.mmdb")
	assert.Error(t, err)
	assert.Nil(t, db)
}
