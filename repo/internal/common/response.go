package common

import (
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"
)

// APIResponse is the standard envelope for all JSON responses.
type APIResponse struct {
	Success bool        `json:"success"`
	Data    interface{} `json:"data,omitempty"`
	Error   *AppError   `json:"error,omitempty"`
	Meta    *Meta       `json:"meta,omitempty"`
}

// Meta holds pagination metadata for list endpoints.
type Meta struct {
	Page       int   `json:"page"`
	PerPage    int   `json:"per_page"`
	Total      int64 `json:"total"`
	TotalPages int   `json:"total_pages"`
}

// Success sends a 200 JSON response with data.
func Success(c *gin.Context, data interface{}) {
	c.JSON(http.StatusOK, APIResponse{
		Success: true,
		Data:    data,
	})
}

// Created sends a 201 JSON response with data.
func Created(c *gin.Context, data interface{}) {
	c.JSON(http.StatusCreated, APIResponse{
		Success: true,
		Data:    data,
	})
}

// SuccessWithMeta sends a 200 JSON response with data and pagination metadata.
func SuccessWithMeta(c *gin.Context, data interface{}, meta *Meta) {
	c.JSON(http.StatusOK, APIResponse{
		Success: true,
		Data:    data,
		Meta:    meta,
	})
}

// RespondError sends a JSON error response using the AppError's status code.
func RespondError(c *gin.Context, err *AppError) {
	c.JSON(err.HTTPStatus, APIResponse{
		Success: false,
		Error:   err,
	})
}

// NoContent sends a 204 response with no body.
func NoContent(c *gin.Context) {
	c.Status(http.StatusNoContent)
}

// PaginationFromQuery extracts page and per_page from query params with defaults.
func PaginationFromQuery(c *gin.Context) (page, perPage int) {
	page = queryInt(c, "page", 1)
	perPage = queryInt(c, "per_page", 20)
	if page < 1 {
		page = 1
	}
	if perPage < 1 {
		perPage = 20
	}
	if perPage > 100 {
		perPage = 100
	}
	return
}

// BuildMeta creates pagination metadata.
func BuildMeta(page, perPage int, total int64) *Meta {
	totalPages := int(total) / perPage
	if int(total)%perPage != 0 {
		totalPages++
	}
	return &Meta{
		Page:       page,
		PerPage:    perPage,
		Total:      total,
		TotalPages: totalPages,
	}
}

func queryInt(c *gin.Context, key string, fallback int) int {
	if v := c.Query(key); v != "" {
		var i int
		if _, err := fmt.Sscanf(v, "%d", &i); err == nil {
			return i
		}
	}
	return fallback
}
