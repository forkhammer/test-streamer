package main

import (
	"log"

	"amka.ru/jit-streamer/api"
	"amka.ru/jit-streamer/config"
	"amka.ru/jit-streamer/services"
)

func main() {
	cfg := config.Load()

	log.Printf("Starting JIT Streamer on port %s", cfg.Port)
	log.Printf("Videos path: %s", cfg.VideosPath)
	log.Printf("Segment duration: %d seconds", cfg.SegmentDuration)

	videoService := services.NewVideoService(cfg)
	segmenter := services.NewSegmenter(cfg.SegmentDuration)
	manifestService := services.NewManifestService(cfg.SegmentDuration)

	defer segmenter.Close()

	router := api.SetupRouter(videoService, segmenter, manifestService)

	if err := router.Run(":" + cfg.Port); err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}
}
