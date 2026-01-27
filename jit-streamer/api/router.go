package api

import (
	"github.com/gin-gonic/gin"

	"amka.ru/jit-streamer/services"
)

func SetupRouter(vs *services.VideoService, seg *services.Segmenter, ms *services.ManifestService) *gin.Engine {
	r := gin.Default()

	// CORS middleware
	r.Use(func(c *gin.Context) {
		c.Header("Access-Control-Allow-Origin", "*")
		c.Header("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
		c.Header("Access-Control-Allow-Headers", "Origin, Content-Type, Accept, Range")
		c.Header("Access-Control-Expose-Headers", "Content-Length, Content-Range")

		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(204)
			return
		}
		c.Next()
	})

	handlers := NewHandlers(vs, seg, ms)

	// API routes
	api := r.Group("/api/v1")
	{
		// Videos management
		api.GET("/videos", handlers.ListVideos)
		api.GET("/videos/:name", handlers.GetVideoInfo)
	}

	// HLS streaming routes (JIT - all generated on the fly)
	hls := r.Group("/hls/:name")
	{
		hls.GET("/master.m3u8", handlers.GetHLSMasterPlaylist)
		hls.GET("/media.m3u8", handlers.GetHLSMediaPlaylist)
		hls.GET("/init.mp4", handlers.GetHLSInitSegment)
		hls.GET("/:segment", handlers.GetHLSSegment)
	}

	// DASH streaming routes (JIT - all generated on the fly)
	dash := r.Group("/dash/:name")
	{
		dash.GET("/stream.mpd", handlers.GetDASHManifest)
		dash.GET("/init.mp4", handlers.GetDASHInitSegment)
		dash.GET("/:segment", handlers.GetDASHSegment)
	}

	// Health check
	r.GET("/health", func(c *gin.Context) {
		c.JSON(200, gin.H{"status": "ok"})
	})

	return r
}
