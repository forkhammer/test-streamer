package config

import (
	"os"
	"strconv"
)

type Config struct {
	Port            string
	VideosPath      string
	SegmentDuration int // seconds
}

func Load() *Config {
	return &Config{
		Port:            getEnv("PORT", "8080"),
		VideosPath:      getEnv("VIDEOS_PATH", "../packager/.videos"),
		SegmentDuration: getEnvInt("SEGMENT_DURATION", 4),
	}
}

func getEnv(key, defaultVal string) string {
	if val := os.Getenv(key); val != "" {
		return val
	}
	return defaultVal
}

func getEnvInt(key string, defaultVal int) int {
	if val := os.Getenv(key); val != "" {
		if i, err := strconv.Atoi(val); err == nil {
			return i
		}
	}
	return defaultVal
}
