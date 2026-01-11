package handler

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
)

func respondError(c *gin.Context, status int, message string) {
	c.JSON(status, gin.H{"error": message})
}

func bindJSON(c *gin.Context, dst interface{}, message string) bool {
	if err := c.ShouldBindJSON(dst); err != nil {
		respondError(c, http.StatusBadRequest, message)
		return false
	}
	return true
}

func parseUintParam(c *gin.Context, key string) (uint, error) {
	raw := c.Param(key)
	id, err := strconv.ParseUint(raw, 10, 32)
	if err != nil {
		return 0, fmt.Errorf("invalid %s", key)
	}
	return uint(id), nil
}

func parseUintQuerySlice(values []string) []uint {
	ids := make([]uint, 0, len(values))
	for _, raw := range values {
		trimmed := strings.TrimSpace(raw)
		if trimmed == "" {
			continue
		}
		parsed, err := strconv.ParseUint(trimmed, 10, 32)
		if err != nil {
			continue
		}
		ids = append(ids, uint(parsed))
	}
	return ids
}
