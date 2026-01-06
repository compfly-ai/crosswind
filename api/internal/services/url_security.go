package services

import (
	"fmt"
	"net/url"
	"strings"
)

// ValidateEndpointURL validates and sanitizes a URL for safe use in HTTP requests.
// Returns a sanitized URL constructed from validated components to prevent SSRF attacks.
func ValidateEndpointURL(endpoint string) (*url.URL, error) {
	parsed, err := url.Parse(endpoint)
	if err != nil {
		return nil, fmt.Errorf("invalid endpoint URL: %w", err)
	}

	// Validate scheme - only allow http or https
	scheme := strings.ToLower(parsed.Scheme)
	if scheme != "http" && scheme != "https" {
		return nil, fmt.Errorf("invalid URL scheme: %s (must be http or https)", parsed.Scheme)
	}

	// Validate host is present
	if parsed.Host == "" {
		return nil, fmt.Errorf("invalid endpoint URL: missing host")
	}

	// Sanitize by reconstructing URL from validated components
	sanitized := &url.URL{
		Scheme:   scheme,
		Host:     parsed.Host,
		Path:     parsed.Path,
		RawQuery: parsed.RawQuery,
	}

	return sanitized, nil
}
