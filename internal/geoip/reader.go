package geoip

import (
	"net"

	"github.com/oschwald/geoip2-golang"
)

// Provider wraps the GeoIP2 database reader to provide country lookup functionality.
type Provider struct {
	db *geoip2.Reader
}

// Open initializes the GeoIP database reader from a specific file path.
func Open(path string) (*Provider, error) {
	db, err := geoip2.Open(path)
	if err != nil {
		return nil, err
	}

	return &Provider{db: db}, nil
}

// Close closes the underlying GeoIP database reader.
func (p *Provider) Close() error {
	return p.db.Close()
}

// GetCountryCode looks up the ISO country code (e.g., "US", "DE") for a given IP address string.
// It returns an empty string if the IP is invalid or the country cannot be determined.
func (p *Provider) GetCountryCode(ipStr string) string {
	ip := net.ParseIP(ipStr)
	if ip == nil {
		return ""
	}

	record, err := p.db.Country(ip)
	if err != nil {
		return ""
	}

	return record.Country.IsoCode
}
