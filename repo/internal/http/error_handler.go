package http

import (
	"errors"
	"log"
	"net/http"

	"propertyops/backend/internal/common"

	"github.com/gin-gonic/gin"
)

// ErrorHandler returns a Gin middleware that converts known error types
// into structured JSON error responses using the common response helpers.
func ErrorHandler() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Next()

		// Process any errors that were added during request handling
		if len(c.Errors) == 0 {
			return
		}

		for _, ginErr := range c.Errors {
			var appErr *common.AppError
			if errors.As(ginErr.Err, &appErr) {
				common.RespondError(c, appErr)
				return
			}
		}

		// If we reach here, there are errors but none are AppError.
		// Log and return a generic 500.
		lastErr := c.Errors.Last()
		requestID, _ := c.Get(string(common.CtxKeyRequestID))
		reqIDStr, _ := requestID.(string)
		log.Printf("ERROR unhandled: %v request_id=%s", lastErr.Err, reqIDStr)

		common.RespondError(c, common.NewInternalError(""))
	}
}

// HandleAppError is a helper that handlers can use to respond with an AppError.
// If the error is nil, it does nothing and returns false.
// If the error is an AppError, it responds and returns true.
func HandleAppError(c *gin.Context, err error) bool {
	if err == nil {
		return false
	}

	var appErr *common.AppError
	if errors.As(err, &appErr) {
		common.RespondError(c, appErr)
		return true
	}

	// Unknown error — log and return 500
	requestID, _ := c.Get(string(common.CtxKeyRequestID))
	reqIDStr, _ := requestID.(string)
	log.Printf("ERROR unhandled: %v request_id=%s", err, reqIDStr)

	c.JSON(http.StatusInternalServerError, common.APIResponse{
		Success: false,
		Error:   common.NewInternalError(""),
	})
	return true
}
