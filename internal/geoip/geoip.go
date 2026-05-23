package geoip

import (
	"fmt"
	"net"

	"github.com/oschwald/geoip2-golang"
)

// DB wraps the MaxMind GeoIP2 database reader.
type DB struct {
	reader *geoip2.Reader
}

// Open opens a MaxMind GeoLite2 MMDB file.
func Open(path string) (*DB, error) {
	r, err := geoip2.Open(path)
	if err != nil {
		return nil, fmt.Errorf("opening geoip db %s: %w", path, err)
	}
	return &DB{reader: r}, nil
}

// Close closes the database reader.
func (db *DB) Close() error {
	return db.reader.Close()
}

// Country returns the ISO 3166 alpha-2 country code for ip.
func (db *DB) Country(ip net.IP) (string, error) {
	record, err := db.reader.Country(ip)
	if err != nil {
		return "", fmt.Errorf("geoip country lookup: %w", err)
	}
	return record.Country.IsoCode, nil
}

// City returns the city name, latitude, and longitude for ip.
func (db *DB) City(ip net.IP) (string, float64, float64, error) {
	record, err := db.reader.City(ip)
	if err != nil {
		return "", 0, 0, fmt.Errorf("geoip city lookup: %w", err)
	}
	city := record.City.Names["en"]
	lat := record.Location.Latitude
	lng := record.Location.Longitude
	return city, lat, lng, nil
}
