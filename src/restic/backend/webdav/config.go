package webdav

import (
	"net/url"
	"strings"

	"restic/errors"
)

// Config contains all configuration necessary to connect to a WebDAV server.
type Config struct {
	URL *url.URL
}

// ParseConfig parses the string s and extracts the REST server URL.
func ParseConfig(s string) (interface{}, error) {
	if !strings.HasPrefix(s, "webdav:") {
		return nil, errors.New("invalid WebDAV backend specification")
	}

	s = s[7:]
	u, err := url.Parse(s)

	if err != nil {
		return nil, errors.Wrap(err, "url.Parse")
	}

	cfg := Config{URL: u}
	return cfg, nil
}
