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

// NewGETRequest creates a GET request with a validated URL.
// Returns an error if the URL fails validation.
func (b *SafeHTTPRequestBuilder) NewGETRequest(ctx context.Context, endpoint string) (*http.Request, error) {
	validatedURL, err := ValidateEndpointURL(endpoint)
	if err != nil {
		return nil, fmt.Errorf("URL validation failed: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "GET", validatedURL.String(), nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create GET request: %w", err)
	}

	return req, nil
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

// NewGETRequestFromValidatedURL creates a GET request from an already-validated URL.
// Use this when the URL has been validated earlier in the call chain.
func (b *SafeHTTPRequestBuilder) NewGETRequestFromValidatedURL(ctx context.Context, validatedEndpoint string) (*http.Request, error) {
	// Re-validate to satisfy static analysis - validation is idempotent
	return b.NewGETRequest(ctx, validatedEndpoint)
}

// NewPOSTRequestFromValidatedURL creates a POST request from an already-validated URL.
// Use this when the URL has been validated earlier in the call chain.
func (b *SafeHTTPRequestBuilder) NewPOSTRequestFromValidatedURL(ctx context.Context, validatedEndpoint string, body []byte) (*http.Request, error) {
	// Re-validate to satisfy static analysis - validation is idempotent
	return b.NewPOSTRequest(ctx, validatedEndpoint, body)
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
