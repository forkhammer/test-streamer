package api

import (
	"amka.ru/packager/services"

	"github.com/gin-gonic/gin"
)

func SetupRouter(packager *services.PackagerService) *gin.Engine {
	router := gin.Default()

	handler := NewHandler(packager)

	api := router.Group("/api/v1")
	{
		api.GET("/videos", handler.ListVideos)

		api.POST("/package", handler.StartPackaging)

		api.GET("/jobs", handler.ListJobs)
		api.GET("/jobs/:id", handler.GetJob)
	}

	return router
}
