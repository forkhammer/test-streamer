package api

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"

	"amka.ru/jit-streamer/services"
)

type Handlers struct {
	videoService    *services.VideoService
	segmenter       *services.Segmenter
	manifestService *services.ManifestService
}

func NewHandlers(vs *services.VideoService, seg *services.Segmenter, ms *services.ManifestService) *Handlers {
	return &Handlers{
		videoService:    vs,
		segmenter:       seg,
		manifestService: ms,
	}
}

// ListVideos returns list of available videos
func (h *Handlers) ListVideos(c *gin.Context) {
	videos, err := h.videoService.ListVideos()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"videos": videos})
}

// GetVideoInfo returns info about specific video
func (h *Handlers) GetVideoInfo(c *gin.Context) {
	name := c.Param("name")
	videoPath, err := h.videoService.GetVideoPath(name)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "video not found"})
		return
	}

	info, err := h.videoService.GetVideoInfo(videoPath)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	info.Name = name
	c.JSON(http.StatusOK, info)
}

// GetHLSMasterPlaylist returns HLS master playlist (generated on the fly)
func (h *Handlers) GetHLSMasterPlaylist(c *gin.Context) {
	name := c.Param("name")

	videoPath, err := h.videoService.GetVideoPath(name)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "video not found"})
		return
	}

	vf, err := h.segmenter.OpenVideo(videoPath)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	durationSec := h.segmenter.GetDurationSec(vf)
	params := services.VideoParams{
		Codec:     vf.VideoCodec,
		Width:     vf.Width,
		Height:    vf.Height,
		Timescale: vf.Timescale,
	}
	playlist := h.manifestService.GenerateHLSMasterPlaylist(name, durationSec, params)

	c.Header("Content-Type", "application/vnd.apple.mpegurl")
	c.Header("Cache-Control", "no-cache")
	c.String(http.StatusOK, playlist)
}

// GetHLSMediaPlaylist returns HLS media playlist (generated on the fly)
func (h *Handlers) GetHLSMediaPlaylist(c *gin.Context) {
	name := c.Param("name")

	videoPath, err := h.videoService.GetVideoPath(name)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "video not found"})
		return
	}

	vf, err := h.segmenter.OpenVideo(videoPath)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	durationSec := h.segmenter.GetDurationSec(vf)
	segmentCount := h.segmenter.GetSegmentCount(vf)
	playlist := h.manifestService.GenerateHLSMediaPlaylist(name, durationSec, segmentCount)

	c.Header("Content-Type", "application/vnd.apple.mpegurl")
	c.Header("Cache-Control", "no-cache")
	c.String(http.StatusOK, playlist)
}

// GetHLSInitSegment returns HLS init segment (generated on the fly)
func (h *Handlers) GetHLSInitSegment(c *gin.Context) {
	name := c.Param("name")

	videoPath, err := h.videoService.GetVideoPath(name)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "video not found"})
		return
	}

	vf, err := h.segmenter.OpenVideo(videoPath)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	data, err := h.segmenter.GenerateInitSegment(vf)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.Header("Content-Type", "video/mp4")
	c.Header("Cache-Control", "max-age=31536000")
	c.Data(http.StatusOK, "video/mp4", data)
}

// GetHLSSegment returns HLS media segment (generated on the fly)
func (h *Handlers) GetHLSSegment(c *gin.Context) {
	name := c.Param("name")
	segment := c.Param("segment")

	// Parse segment number from segment_N.m4s
	segmentNum := 0
	if strings.HasPrefix(segment, "segment_") && strings.HasSuffix(segment, ".m4s") {
		numStr := strings.TrimSuffix(strings.TrimPrefix(segment, "segment_"), ".m4s")
		if n, err := strconv.Atoi(numStr); err == nil {
			segmentNum = n
		}
	}

	videoPath, err := h.videoService.GetVideoPath(name)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "video not found"})
		return
	}

	vf, err := h.segmenter.OpenVideo(videoPath)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	data, err := h.segmenter.GenerateMediaSegment(vf, segmentNum)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.Header("Content-Type", "video/mp4")
	c.Header("Cache-Control", "max-age=31536000")
	c.Data(http.StatusOK, "video/mp4", data)
}

// GetDASHManifest returns DASH MPD manifest (generated on the fly)
func (h *Handlers) GetDASHManifest(c *gin.Context) {
	name := c.Param("name")

	videoPath, err := h.videoService.GetVideoPath(name)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "video not found"})
		return
	}

	vf, err := h.segmenter.OpenVideo(videoPath)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	durationSec := h.segmenter.GetDurationSec(vf)
	segmentCount := h.segmenter.GetSegmentCount(vf)
	params := services.VideoParams{
		Codec:     vf.VideoCodec,
		Width:     vf.Width,
		Height:    vf.Height,
		Timescale: vf.Timescale,
	}
	mpd, err := h.manifestService.GenerateDASHMPD(name, durationSec, segmentCount, params)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.Header("Content-Type", "application/dash+xml")
	c.Header("Cache-Control", "no-cache")
	c.String(http.StatusOK, mpd)
}

// GetDASHInitSegment returns DASH init segment (generated on the fly)
func (h *Handlers) GetDASHInitSegment(c *gin.Context) {
	h.GetHLSInitSegment(c) // Same format for fMP4
}

// GetDASHSegment returns DASH media segment (generated on the fly)
func (h *Handlers) GetDASHSegment(c *gin.Context) {
	name := c.Param("name")
	segment := c.Param("segment")

	// Handle init.mp4
	if segment == "init.mp4" {
		h.GetDASHInitSegment(c)
		return
	}

	// Parse segment number from segment_N.m4s
	segmentNum := 0
	if strings.HasPrefix(segment, "segment_") && strings.HasSuffix(segment, ".m4s") {
		numStr := strings.TrimSuffix(strings.TrimPrefix(segment, "segment_"), ".m4s")
		if n, err := strconv.Atoi(numStr); err == nil {
			segmentNum = n
		}
	}

	videoPath, err := h.videoService.GetVideoPath(name)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "video not found"})
		return
	}

	vf, err := h.segmenter.OpenVideo(videoPath)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	data, err := h.segmenter.GenerateMediaSegment(vf, segmentNum)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.Header("Content-Type", "video/mp4")
	c.Header("Cache-Control", "max-age=31536000")
	c.Data(http.StatusOK, "video/mp4", data)
}
