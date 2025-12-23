package handlers

import (
	"net/http"

	"github.com/compfly-ai/crosswind/internal/services"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// DatasetHandlers handles dataset-related HTTP requests
type DatasetHandlers struct {
	services *services.Services
	logger   *zap.Logger
}

// NewDatasetHandlers creates a new dataset handlers instance
func NewDatasetHandlers(svc *services.Services, logger *zap.Logger) *DatasetHandlers {
	return &DatasetHandlers{
		services: svc,
		logger:   logger,
	}
}

// List handles GET /datasets
func (h *DatasetHandlers) List(c *gin.Context) {
	category := c.Query("category")
	isActive := c.DefaultQuery("isActive", "true") == "true"

	response, err := h.services.Dataset.List(c.Request.Context(), category, isActive)
	if err != nil {
		h.logger.Error("failed to list datasets", zap.Error(err))
		respondWithError(c, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to list datasets", nil)
		return
	}

	c.JSON(http.StatusOK, response)
}

// Get handles GET /datasets/:datasetId
func (h *DatasetHandlers) Get(c *gin.Context) {
	datasetID := c.Param("datasetId")

	dataset, err := h.services.Dataset.Get(c.Request.Context(), datasetID)
	if err != nil {
		if err == services.ErrDatasetNotFound {
			respondWithError(c, http.StatusNotFound, "DATASET_NOT_FOUND", "Dataset not found", gin.H{"datasetId": datasetID})
			return
		}
		h.logger.Error("failed to get dataset", zap.Error(err))
		respondWithError(c, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to get dataset", nil)
		return
	}

	c.JSON(http.StatusOK, dataset)
}

