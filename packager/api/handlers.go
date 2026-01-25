package api

import (
	"net/http"

	"amka.ru/packager/services"

	"github.com/gin-gonic/gin"
)

type Handler struct {
	packager *services.PackagerService
}

func NewHandler(packager *services.PackagerService) *Handler {
	return &Handler{
		packager: packager,
	}
}

type PackageRequest struct {
	VideoName string `json:"video_name" binding:"required"`
}

type ErrorResponse struct {
	Error string `json:"error"`
}

func (h *Handler) ListVideos(c *gin.Context) {
	videos, err := h.packager.ListVideos()
	if err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"videos": videos})
}

func (h *Handler) StartPackaging(c *gin.Context) {
	var req PackageRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: "video_name is required"})
		return
	}

	job, err := h.packager.StartPackaging(req.VideoName)
	if err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: err.Error()})
		return
	}

	c.JSON(http.StatusAccepted, job)
}

func (h *Handler) GetJob(c *gin.Context) {
	jobID := c.Param("id")

	job, found := h.packager.GetJob(jobID)
	if !found {
		c.JSON(http.StatusNotFound, ErrorResponse{Error: "job not found"})
		return
	}

	c.JSON(http.StatusOK, job)
}

func (h *Handler) ListJobs(c *gin.Context) {
	jobs := h.packager.ListJobs()
	c.JSON(http.StatusOK, gin.H{"jobs": jobs})
}
