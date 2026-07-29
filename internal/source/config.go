package source

import (
	"encoding/json"
	"strconv"
	"strings"

	"faangjobs/internal/registry"
)

// configStr reads a string option from a company's Config, falling back to def.
func configStr(c registry.Company, key, def string) string {
	if c.Config != nil {
		if v, ok := c.Config[key]; ok && strings.TrimSpace(v) != "" {
			return strings.TrimSpace(v)
		}
	}
	return def
}

// configInt reads an integer option from a company's Config, falling back to def.
func configInt(c registry.Company, key string, def int) int {
	if c.Config != nil {
		if v, ok := c.Config[key]; ok {
			if n, err := strconv.Atoi(strings.TrimSpace(v)); err == nil {
				return n
			}
		}
	}
	return def
}

// configJSON unmarshals a JSON-encoded config value into out; returns false if
// the key is absent or invalid.
func configJSON(c registry.Company, key string, out any) bool {
	if c.Config == nil {
		return false
	}
	v, ok := c.Config[key]
	if !ok || strings.TrimSpace(v) == "" {
		return false
	}
	return json.Unmarshal([]byte(v), out) == nil
}
