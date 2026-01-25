package main

import (
	"log"
	"os"

	"amka.ru/packager/api"
	"amka.ru/packager/models"
	"amka.ru/packager/services"
)

const (
	defaultVideosDir    = ".videos"
	defaultPlaylistsDir = ".playlists"
	defaultPort         = "8080"
)

func main() {
	videosDir := getEnv("VIDEOS_DIR", defaultVideosDir)
	playlistsDir := getEnv("PLAYLISTS_DIR", defaultPlaylistsDir)
	port := getEnv("PORT", defaultPort)

	if err := ensureDir(videosDir); err != nil {
		log.Fatalf("Failed to create videos directory: %v", err)
	}

	if err := ensureDir(playlistsDir); err != nil {
		log.Fatalf("Failed to create playlists directory: %v", err)
	}

	jobStore := models.NewJobStore()

	packagerService := services.NewPackagerService(videosDir, playlistsDir, jobStore)

	router := api.SetupRouter(packagerService)

	log.Printf("Starting server on port %s", port)
	log.Printf("Videos directory: %s", videosDir)
	log.Printf("Playlists directory: %s", playlistsDir)

	if err := router.Run(":" + port); err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func ensureDir(path string) error {
	return os.MkdirAll(path, 0755)
}
