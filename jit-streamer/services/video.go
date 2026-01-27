package services

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"amka.ru/jit-streamer/config"
	"amka.ru/jit-streamer/models"
)

type VideoService struct {
	cfg *config.Config
}

func NewVideoService(cfg *config.Config) *VideoService {
	return &VideoService{cfg: cfg}
}

func (s *VideoService) ListVideos() ([]models.VideoInfo, error) {
	entries, err := os.ReadDir(s.cfg.VideosPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read videos directory: %w", err)
	}

	var videos []models.VideoInfo
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		ext := strings.ToLower(filepath.Ext(entry.Name()))
		if ext != ".mp4" && ext != ".avi" && ext != ".mkv" && ext != ".mov" {
			continue
		}

		videoPath := filepath.Join(s.cfg.VideosPath, entry.Name())
		info, err := s.GetVideoInfo(videoPath)
		if err != nil {
			continue
		}
		info.Name = strings.TrimSuffix(entry.Name(), ext)
		videos = append(videos, *info)
	}
	return videos, nil
}

func (s *VideoService) GetVideoInfo(path string) (*models.VideoInfo, error) {
	cmd := exec.Command("ffprobe",
		"-v", "quiet",
		"-print_format", "json",
		"-show_format",
		"-show_streams",
		path,
	)

	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("ffprobe failed: %w", err)
	}

	var probe struct {
		Streams []struct {
			CodecType string  `json:"codec_type"`
			CodecName string  `json:"codec_name"`
			Width     int     `json:"width"`
			Height    int     `json:"height"`
			FrameRate string  `json:"r_frame_rate"`
		} `json:"streams"`
		Format struct {
			Duration string `json:"duration"`
			BitRate  string `json:"bit_rate"`
		} `json:"format"`
	}

	if err := json.Unmarshal(output, &probe); err != nil {
		return nil, fmt.Errorf("failed to parse ffprobe output: %w", err)
	}

	info := &models.VideoInfo{Path: path}

	for _, stream := range probe.Streams {
		if stream.CodecType == "video" {
			info.Width = stream.Width
			info.Height = stream.Height
			info.Codec = stream.CodecName
			if parts := strings.Split(stream.FrameRate, "/"); len(parts) == 2 {
				num, _ := strconv.ParseFloat(parts[0], 64)
				den, _ := strconv.ParseFloat(parts[1], 64)
				if den > 0 {
					info.FrameRate = num / den
				}
			}
			break
		}
	}

	if durationSec, err := strconv.ParseFloat(probe.Format.Duration, 64); err == nil {
		info.Duration = time.Duration(durationSec * float64(time.Second))
	}
	if bitrate, err := strconv.ParseInt(probe.Format.BitRate, 10, 64); err == nil {
		info.Bitrate = bitrate
	}

	return info, nil
}

func (s *VideoService) GetVideoPath(name string) (string, error) {
	entries, err := os.ReadDir(s.cfg.VideosPath)
	if err != nil {
		return "", fmt.Errorf("failed to read videos directory: %w", err)
	}

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		ext := filepath.Ext(entry.Name())
		baseName := strings.TrimSuffix(entry.Name(), ext)
		if baseName == name {
			return filepath.Join(s.cfg.VideosPath, entry.Name()), nil
		}
	}
	return "", fmt.Errorf("video not found: %s", name)
}
