package models

import "time"

type VideoInfo struct {
	Name      string        `json:"name"`
	Path      string        `json:"-"`
	Duration  time.Duration `json:"duration"`
	Width     int           `json:"width"`
	Height    int           `json:"height"`
	Bitrate   int64         `json:"bitrate"`
	Codec     string        `json:"codec"`
	FrameRate float64       `json:"frame_rate"`
}

type StreamInfo struct {
	VideoID         string `json:"video_id"`
	SegmentDuration int    `json:"segment_duration"`
	TotalSegments   int    `json:"total_segments"`
	Qualities       []Quality `json:"qualities"`
}

type Quality struct {
	Name       string `json:"name"`
	Width      int    `json:"width"`
	Height     int    `json:"height"`
	Bitrate    int    `json:"bitrate"`
	Bandwidth  int    `json:"bandwidth"`
}

var DefaultQualities = []Quality{
	{Name: "360p", Width: 640, Height: 360, Bitrate: 800000, Bandwidth: 856000},
	{Name: "480p", Width: 854, Height: 480, Bitrate: 1400000, Bandwidth: 1498000},
	{Name: "720p", Width: 1280, Height: 720, Bitrate: 2800000, Bandwidth: 2996000},
	{Name: "1080p", Width: 1920, Height: 1080, Bitrate: 5000000, Bandwidth: 5350000},
}
