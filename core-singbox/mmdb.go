package main

import (
	"net"
	"path/filepath"

	"github.com/oschwald/maxminddb-golang"
)

var mmdbOnce struct {
	once   chan struct{}
	reader *maxminddb.Reader
	err    error
}

func init() {
	mmdbOnce.once = make(chan struct{}, 1)
}

// getMMDB lazily opens the Country.mmdb inside homeDir (extracted from assets
// by the Dart side).
func getMMDB(homeDir string) *maxminddb.Reader {
	select {
	case mmdbOnce.once <- struct{}{}:
		defer func() { <-mmdbOnce.once }()
		if mmdbOnce.reader != nil {
			return mmdbOnce.reader
		}
		path := filepath.Join(homeDir, "Country.mmdb")
		reader, err := maxminddb.Open(path)
		if err != nil {
			mmdbOnce.err = err
			return nil
		}
		mmdbOnce.reader = reader
		return mmdbOnce.reader
	default:
		return mmdbOnce.reader
	}
}

type countryRecord struct {
	Country struct {
		ISOCode string `maxminddb:"iso_code"`
	} `maxminddb:"country"`
}

func handleGetCountryCode(ip string, fn func(value string)) {
	go func() {
		reader := getMMDB(engine.homeDir)
		if reader == nil {
			fn("")
			return
		}
		parsed := net.ParseIP(ip)
		if parsed == nil {
			fn("")
			return
		}
		var record countryRecord
		if err := reader.Lookup(parsed, &record); err != nil {
			fn("")
			return
		}
		fn(record.Country.ISOCode)
	}()
}
