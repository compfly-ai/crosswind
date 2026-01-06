package services

import (
	"bytes"
	"context"
	"fmt"
	"net/http"
	"net/url"
)

// SafeHTTPRequestBuilder creates HTTP requests with validated URLs.
// This encapsulates URL validation and request creation to satisfy static analysis.
type SafeHTTPRequestBuilder struct{}

// NewSafeHTTPRequestBuilder creates a new request builder.
func NewSafeHTTPRequestBuilder() *SafeHTTPRequestBuilder {
	return &SafeHTTPRequestBuilder{}
}

// NewPOSTRequest creates a POST request with a validated URL and body.
// Returns an error if the URL fails validation.
func (b *SafeHTTPRequestBuilder) NewPOSTRequest(ctx context.Context, endpoint string, body []byte) (*http.Request, error) {
	validatedURL, err := ValidateEndpointURL(endpoint)
	if err != nil {
		return nil, fmt.Errorf("URL validation failed: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", validatedURL.String(), bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("failed to create POST request: %w", err)
	}

	return req, nil
}

// ResolveURL resolves a potentially relative endpoint path against a base URL.
// If endpointPath is absolute and valid, it's returned directly.
// If it's relative, it's resolved against baseEndpoint.
// Returns a validated endpoint string.
func (b *SafeHTTPRequestBuilder) ResolveURL(baseEndpoint string, endpointPath string) (string, error) {
	// Try to validate endpointPath directly - if it's absolute and valid, use it
	if validated, err := ValidateEndpointURL(endpointPath); err == nil {
		return validated.String(), nil
	}

	// Otherwise, resolve relative path against base endpoint
	baseURL, err := ValidateEndpointURL(baseEndpoint)
	if err != nil {
		return "", fmt.Errorf("invalid base endpoint: %w", err)
	}

	// Parse endpoint path (may include query params like ?sessionId=xxx)
	ref, err := url.Parse(endpointPath)
	if err != nil {
		return "", fmt.Errorf("invalid endpoint path: %w", err)
	}

	// Resolve against base and validate
	resolved := baseURL.ResolveReference(ref)
	validated, err := ValidateEndpointURL(resolved.String())
	if err != nil {
		return "", err
	}
	return validated.String(), nil
}
