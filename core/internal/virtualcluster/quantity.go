package virtualcluster

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
)

// Bytes is a byte count that unmarshals from a number or a string with a
// binary (Ki, Mi, Gi, Ti, Pi) or decimal (K, M, G, T, P) suffix, e.g. "64Gi".
type Bytes uint64

var byteSuffixes = []struct {
	suffix string
	mult   uint64
}{
	{"Pi", 1 << 50}, {"Ti", 1 << 40}, {"Gi", 1 << 30}, {"Mi", 1 << 20}, {"Ki", 1 << 10},
	{"P", 1e15}, {"T", 1e12}, {"G", 1e9}, {"M", 1e6}, {"K", 1e3},
}

// ParseBytes parses a quantity such as "512Mi", "2Ti", or "1000000".
func ParseBytes(s string) (Bytes, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0, fmt.Errorf("empty quantity")
	}
	mult := uint64(1)
	num := s
	for _, sfx := range byteSuffixes {
		if strings.HasSuffix(s, sfx.suffix) {
			mult = sfx.mult
			num = strings.TrimSuffix(s, sfx.suffix)
			break
		}
	}
	v, err := strconv.ParseFloat(strings.TrimSpace(num), 64)
	if err != nil || v < 0 {
		return 0, fmt.Errorf("invalid quantity %q", s)
	}
	return Bytes(v * float64(mult)), nil
}

// UnmarshalJSON implements json.Unmarshaler (sigs.k8s.io/yaml routes through
// JSON).
func (b *Bytes) UnmarshalJSON(data []byte) error {
	var s string
	if err := json.Unmarshal(data, &s); err == nil {
		v, err := ParseBytes(s)
		if err != nil {
			return err
		}
		*b = v
		return nil
	}
	var n uint64
	if err := json.Unmarshal(data, &n); err != nil {
		return fmt.Errorf("quantity must be a number or a string: %s", data)
	}
	*b = Bytes(n)
	return nil
}
