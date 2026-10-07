// Package helpers provides shared utility functions for HTTP handlers.
package helpers

import (
	"context"

	"github.com/gin-gonic/gin"
)

// RequestContext extracts a context.Context from a Gin request context.
// The returned context carries the request's cancellation, deadlines, and
// values such as trace IDs. Use this when calling service-layer methods
// that accept context.Context as their first argument.
func RequestContext(c *gin.Context) context.Context {
	return c.Request.Context()
}
