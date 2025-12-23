package middleware

import (
	"crypto/subtle"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

const (
	// ContextKeyOrgID is the key used to store the org ID in the context
	ContextKeyOrgID = "orgId"
)

// AuthConfig holds configuration for the auth middleware
type AuthConfig struct {
	APIKey string
}

// Auth returns a middleware that validates API keys
func Auth(authCfg *AuthConfig) gin.HandlerFunc {
	return func(c *gin.Context) {
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"error": gin.H{
					"code":    "INVALID_API_KEY",
					"message": "Authorization header is required",
				},
			})
			return
		}

		// Extract API key from Bearer token
		parts := strings.SplitN(authHeader, " ", 2)
		if len(parts) != 2 || strings.ToLower(parts[0]) != "bearer" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"error": gin.H{
					"code":    "INVALID_API_KEY",
					"message": "Invalid Authorization header format. Use: Bearer <api_key>",
				},
			})
			return
		}

		apiKey := parts[1]
		if apiKey == "" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"error": gin.H{
					"code":    "INVALID_API_KEY",
					"message": "API key is required",
				},
			})
			return
		}

		// Simple API key validation
		// Use constant-time comparison to prevent timing attacks
		if authCfg == nil || authCfg.APIKey == "" || subtle.ConstantTimeCompare([]byte(apiKey), []byte(authCfg.APIKey)) != 1 {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"error": gin.H{
					"code":    "INVALID_API_KEY",
					"message": "Invalid API key",
				},
			})
			return
		}

		c.Next()
	}
}

// GetOrgID retrieves the org ID from the context
func GetOrgID(c *gin.Context) string {
	orgID, _ := c.Get(ContextKeyOrgID)
	if orgID == nil {
		return ""
	}
	return orgID.(string)
}

// BasicAuth returns a middleware for HTTP Basic Authentication
// Used to protect documentation endpoints (/docs, /openapi.yaml)
func BasicAuth(username, password string) gin.HandlerFunc {
	return func(c *gin.Context) {
		// If no password configured, allow access (dev mode)
		if password == "" {
			c.Next()
			return
		}

		user, pass, hasAuth := c.Request.BasicAuth()
		if !hasAuth {
			c.Header("WWW-Authenticate", `Basic realm="API Documentation"`)
			c.AbortWithStatus(http.StatusUnauthorized)
			return
		}

		// Constant-time comparison to prevent timing attacks
		usernameMatch := subtle.ConstantTimeCompare([]byte(user), []byte(username)) == 1
		passwordMatch := subtle.ConstantTimeCompare([]byte(pass), []byte(password)) == 1

		if !usernameMatch || !passwordMatch {
			c.Header("WWW-Authenticate", `Basic realm="API Documentation"`)
			c.AbortWithStatus(http.StatusUnauthorized)
			return
		}

		c.Next()
	}
}
