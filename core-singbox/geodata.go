package main

import (
	"bytes"
	"compress/gzip"
	"encoding/binary"
	"io"
	"net/netip"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

func joinPath(base string, name string) string {
	return filepath.Join(base, name)
}

// Minimal protobuf wire parser for the v2ray-style geoip.dat / geosite.dat
// shipped in the app assets (copied to homeDir by the Dart side).
//
// geoip.dat: repeated GeoIP { string country_code = 1; repeated CIDR cidr = 2; }
//            CIDR { bytes ip = 1; uint32 prefix = 2; }
// geosite.dat: repeated Geosite { string name = 1; repeated Domain domain = 2; }
//            Domain { Type type = 1; string value = 2; } Type: Plain=0 Regex=1 Domain=2 Full=3

type geoEntry struct {
	countryCode string
	prefixes    []netip.Prefix
}

type domainEntry struct {
	name  string
	value string
	dtype int // 0 plain, 1 regex, 2 domain, 3 full
}

var (
	geoDataOnce     sync.Once
	geoDataByCC     map[string][]netip.Prefix
	geositeDataOnce sync.Once
	geositeData     map[string][]domainEntry
)

func readMaybeGzip(path string) ([]byte, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	if len(data) > 2 && data[0] == 0x1f && data[1] == 0x8b {
		reader, err := gzip.NewReader(bytes.NewReader(data))
		if err != nil {
			return nil, err
		}
		return io.ReadAll(reader)
	}
	return data, nil
}

// protoField is one decoded wire field.
type protoField struct {
	number int
	wire   int
	varint uint64
	bytes  []byte
}

func parseProtoFields(data []byte) []protoField {
	var fields []protoField
	for len(data) > 0 {
		tag, n := binary.Uvarint(data)
		if n <= 0 {
			break
		}
		data = data[n:]
		fieldNumber := int(tag >> 3)
		wireType := int(tag & 0x7)
		switch wireType {
		case 0: // varint
			value, n := binary.Uvarint(data)
			if n <= 0 {
				return fields
			}
			data = data[n:]
			fields = append(fields, protoField{number: fieldNumber, wire: 0, varint: value})
		case 1: // 64-bit
			if len(data) < 8 {
				return fields
			}
			fields = append(fields, protoField{number: fieldNumber, wire: 1, bytes: data[:8]})
			data = data[8:]
		case 2: // length-delimited
			length, n := binary.Uvarint(data)
			if n <= 0 || uint64(len(data)-n) < length {
				return fields
			}
			fields = append(fields, protoField{number: fieldNumber, wire: 2, bytes: data[n : n+int(length)]})
			data = data[n+int(length):]
		case 5: // 32-bit
			if len(data) < 4 {
				return fields
			}
			fields = append(fields, protoField{number: fieldNumber, wire: 5, bytes: data[:4]})
			data = data[4:]
		default:
			return fields
		}
	}
	return fields
}

func loadGeoData(homeDir string) map[string][]netip.Prefix {
	geoDataOnce.Do(func() {
		geoDataByCC = map[string][]netip.Prefix{}
		data, err := readMaybeGzip(joinPath(homeDir, "GEOIP.dat"))
		if err != nil {
			logError("read GEOIP.dat: %v", err)
			return
		}
		for _, field := range parseProtoFields(data) {
			if field.number != 1 {
				continue
			}
			var cc string
			var prefixes []netip.Prefix
			for _, sub := range parseProtoFields(field.bytes) {
				switch sub.number {
				case 1:
					cc = string(sub.bytes)
				case 2:
					var ip []byte
					var bits uint32
					for _, cidrField := range parseProtoFields(sub.bytes) {
						switch cidrField.number {
						case 1:
							ip = cidrField.bytes
						case 2:
							bits = uint32(cidrField.varint)
						}
					}
					if addr, ok := netip.AddrFromSlice(ip); ok {
						prefix := netip.PrefixFrom(addr, int(bits))
						prefixes = append(prefixes, prefix.Masked())
					}
				}
			}
			if cc != "" {
				geoDataByCC[strings.ToUpper(cc)] = prefixes
			}
		}
	})
	return geoDataByCC
}

func loadGeositeData(homeDir string) map[string][]domainEntry {
	geositeDataOnce.Do(func() {
		geositeData = map[string][]domainEntry{}
		data, err := readMaybeGzip(joinPath(homeDir, "GEOSITE.dat"))
		if err != nil {
			logError("read GEOSITE.dat: %v", err)
			return
		}
		for _, field := range parseProtoFields(data) {
			if field.number != 1 {
				continue
			}
			var name string
			var domains []domainEntry
			for _, sub := range parseProtoFields(field.bytes) {
				switch sub.number {
				case 1:
					name = string(sub.bytes)
				case 2:
					entry := domainEntry{}
					for _, domainField := range parseProtoFields(sub.bytes) {
						switch domainField.number {
						case 1:
							entry.dtype = int(domainField.varint)
						case 2:
							entry.value = string(domainField.bytes)
						}
					}
					if entry.value != "" {
						domains = append(domains, entry)
					}
				}
			}
			if name != "" {
				geositeData[strings.ToLower(name)] = domains
			}
		}
	})
	return geositeData
}

// expandGeoIPPrefixes returns the CIDR prefixes for a country/region code
// (GEOIP rule payload). Supports mihomo's "LAN" alias → private addresses.
func expandGeoIPPrefixes(homeDir string, code string) []netip.Prefix {
	code = strings.ToUpper(strings.TrimPrefix(code, "!"))
	if code == "LAN" {
		code = "PRIVATE"
	}
	data := loadGeoData(homeDir)
	return data[code]
}

// expandGeosite returns the domain entries for a geosite category.
func expandGeosite(homeDir string, category string) []domainEntry {
	data := loadGeositeData(homeDir)
	entries, ok := data[strings.ToLower(category)]
	if !ok {
		// Support attribute form "category@attr" by ignoring the attribute.
		if idx := strings.Index(category, "@"); idx > 0 {
			entries, ok = data[strings.ToLower(category[:idx])]
		}
	}
	return entries
}
