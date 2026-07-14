package api

import (
	"errors"
	"fmt"
	"net"
	"regexp"
	"strings"
)

var domainLabelRe = regexp.MustCompile(`^[a-z0-9]([a-z0-9-]{0,61}[a-z0-9])?$`)

// normalizeDomain trims whitespace, lowercases, strips an http(s):// prefix,
// strips any path/port suffix and trailing dot, then validates the result is
// a plausible DNS hostname. Returns the cleaned domain or an error.
func normalizeDomain(raw string) (string, error) {
	d := strings.ToLower(strings.TrimSpace(raw))
	d = strings.TrimPrefix(d, "https://")
	d = strings.TrimPrefix(d, "http://")
	if i := strings.IndexAny(d, "/?#"); i >= 0 {
		d = d[:i]
	}
	if i := strings.LastIndex(d, ":"); i >= 0 && strings.Trim(d[i+1:], "0123456789") == "" {
		d = d[:i]
	}
	d = strings.TrimSuffix(d, ".")

	if d == "" {
		return "", errors.New("domain is required")
	}
	if net.ParseIP(d) != nil {
		return "", errors.New("use a domain name, not an IP address")
	}
	if len(d) > 253 {
		return "", errors.New("domain is too long (max 253 characters)")
	}
	if !strings.Contains(d, ".") {
		return "", fmt.Errorf("%q is not a valid domain: it must contain a dot (e.g. example.com)", d)
	}
	for _, label := range strings.Split(d, ".") {
		if !domainLabelRe.MatchString(label) {
			return "", fmt.Errorf("%q is not a valid domain name", d)
		}
	}
	return d, nil
}
