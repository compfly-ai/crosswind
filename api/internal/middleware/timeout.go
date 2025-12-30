package middleware

import (
	"context"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
)

// Timeout returns a middleware that enforces a per-request timeout for all routes
// except those explicitly exempted (e.g., long-lived SSE streams).
func Timeout(defaultTimeout time.Duration, exemptPaths ...string) gin.HandlerFunc {
	exempt := make(map[string]struct{}, len(exemptPaths))
	for _, p := range exemptPaths {
		exempt[p] = struct{}{}
	}

	return func(c *gin.Context) {
		if defaultTimeout <= 0 {
			c.Next()
			return
		}

		path := c.FullPath()
		if path == "" {
			path = c.Request.URL.Path
		}
		if _, ok := exempt[path]; ok {
			c.Next()
			return
		}

		ctx, cancel := context.WithTimeout(c.Request.Context(), defaultTimeout)
		defer cancel()

		c.Request = c.Request.WithContext(ctx)
		c.Next()

		// If the context timed out and nothing was written, return a 504
		if ctx.Err() == context.DeadlineExceeded && !c.Writer.Written() {
			c.AbortWithStatusJSON(http.StatusGatewayTimeout, gin.H{
				"error": gin.H{
					"code":    "REQUEST_TIMEOUT",
					"message": "request exceeded time limit",
				},
			})
		}
	}
}
