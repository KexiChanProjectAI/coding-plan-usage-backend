// Package api provides HTTP API handlers.
package api

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/quotahub/ucpqa/internal/domain"
	"github.com/quotahub/ucpqa/internal/infrastructure/store"
)

// UsageResponse wraps an AccountSnapshot with a derived status field.
type UsageResponse struct {
	domain.AccountSnapshot
	Status domain.Status `json:"status"`
}

// UsageHandler handles usage-related HTTP requests.
type UsageHandler struct {
	store           *store.Store
	maxStaleDuration time.Duration
}

// NewUsageHandler creates a new UsageHandler.
func NewUsageHandler(store *store.Store, maxStaleDuration time.Duration) *UsageHandler {
	return &UsageHandler{
		store:           store,
		maxStaleDuration: maxStaleDuration,
	}
}

// GetUsage handles GET /api/v1/usage.
// It returns a JSON array of all account snapshots with their derived status.
func (h *UsageHandler) GetUsage(c *gin.Context) {
	platforms := h.store.Platforms()

	// Fresh-start empty store: return initializing status
	if len(platforms) == 0 {
		c.JSON(http.StatusOK, []UsageResponse{
			{
				AccountSnapshot: domain.AccountSnapshot{
					Platform: "",
					Version:  0,
				},
				Status: domain.StatusInitializing,
			},
		})
		return
	}

	responses := make([]UsageResponse, 0, len(platforms))
	for _, platform := range platforms {
		snapshot, ok := h.store.Get(platform)
		if !ok {
			continue
		}
		status := domain.DeriveStatus(snapshot, h.maxStaleDuration)
		responses = append(responses, UsageResponse{
			AccountSnapshot: snapshot,
			Status:          status,
		})
	}

	c.JSON(http.StatusOK, responses)
}

// RegisterRoutes registers the usage handler routes on the given router group.
func (h *UsageHandler) RegisterRoutes(rg *gin.RouterGroup) {
	rg.GET("/usage", h.GetUsage)
}