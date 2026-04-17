package middleware

import (
	"net/http"
	"runtime/debug"

	"go-job/pkg/logger"
	"go-job/pkg/response"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// Recovery catches panic and converts it to unified API response.
func Recovery() gin.HandlerFunc {
	return func(c *gin.Context) {
		defer func() {
			if r := recover(); r != nil {
				logger.L().Error("panic recovered",
					zap.Any("panic", r),
					zap.ByteString("stack", debug.Stack()),
					zap.String("path", c.Request.URL.Path),
					zap.String("method", c.Request.Method),
				)
				response.JSON(c, http.StatusInternalServerError, 50000, "internal server error", nil)
				c.Abort()
			}
		}()
		c.Next()
	}
}
