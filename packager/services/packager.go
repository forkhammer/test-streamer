package services

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"amka.ru/packager/models"

	"github.com/google/uuid"
)

type Resolution struct {
	Name   string
	Width  int
	Height int
}

var Resolutions = []Resolution{
	{Name: "360p", Width: 640, Height: 360},
	{Name: "480p", Width: 854, Height: 480},
	{Name: "720p", Width: 1280, Height: 720},
	{Name: "1080p", Width: 1920, Height: 1080},
}

type PackagerService struct {
	videosDir    string
	playlistsDir string
	jobStore     *models.JobStore
}

func NewPackagerService(videosDir, playlistsDir string, jobStore *models.JobStore) *PackagerService {
	return &PackagerService{
		videosDir:    videosDir,
		playlistsDir: playlistsDir,
		jobStore:     jobStore,
	}
}

func (s *PackagerService) VideoExists(videoName string) bool {
	videoPath := filepath.Join(s.videosDir, videoName)
	_, err := os.Stat(videoPath)
	return err == nil
}

func (s *PackagerService) ListVideos() ([]string, error) {
	entries, err := os.ReadDir(s.videosDir)
	if err != nil {
		return nil, err
	}

	var videos []string
	for _, entry := range entries {
		if !entry.IsDir() && strings.HasSuffix(strings.ToLower(entry.Name()), ".mp4") {
			videos = append(videos, entry.Name())
		}
	}
	return videos, nil
}

func (s *PackagerService) StartPackaging(videoName string) (*models.Job, error) {
	if !s.VideoExists(videoName) {
		return nil, fmt.Errorf("video file not found: %s", videoName)
	}

	job := &models.Job{
		ID:        uuid.New().String(),
		VideoName: videoName,
		Status:    models.JobStatusPending,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	s.jobStore.Create(job)

	go s.processVideo(job)

	return job, nil
}

func (s *PackagerService) GetJob(id string) (*models.Job, bool) {
	return s.jobStore.Get(id)
}

func (s *PackagerService) ListJobs() []*models.Job {
	return s.jobStore.List()
}

func (s *PackagerService) processVideo(job *models.Job) {
	job.Status = models.JobStatusProcessing
	s.jobStore.Update(job)

	videoName := strings.TrimSuffix(job.VideoName, filepath.Ext(job.VideoName))
	inputPath := filepath.Join(s.videosDir, job.VideoName)
	outputBaseDir := filepath.Join(s.playlistsDir, videoName)

	hlsDir := filepath.Join(outputBaseDir, "hls")
	dashDir := filepath.Join(outputBaseDir, "dash")

	if err := os.MkdirAll(hlsDir, 0755); err != nil {
		s.failJob(job, fmt.Errorf("failed to create HLS directory: %w", err))
		return
	}

	if err := os.MkdirAll(dashDir, 0755); err != nil {
		s.failJob(job, fmt.Errorf("failed to create DASH directory: %w", err))
		return
	}

	if err := s.packageHLS(inputPath, hlsDir); err != nil {
		s.failJob(job, fmt.Errorf("HLS packaging failed: %w", err))
		return
	}

	if err := s.packageDASH(inputPath, dashDir); err != nil {
		s.failJob(job, fmt.Errorf("DASH packaging failed: %w", err))
		return
	}

	job.Status = models.JobStatusCompleted
	s.jobStore.Update(job)
	log.Printf("Job %s completed successfully", job.ID)
}

func (s *PackagerService) failJob(job *models.Job, err error) {
	job.Status = models.JobStatusFailed
	job.Error = err.Error()
	s.jobStore.Update(job)
	log.Printf("Job %s failed: %v", job.ID, err)
}

func (s *PackagerService) packageHLS(inputPath, outputDir string) error {
	args := []string{
		"-i", inputPath,
		"-y",
	}

	var filterComplex strings.Builder
	filterComplex.WriteString("[0:v]split=4")
	for i := range Resolutions {
		filterComplex.WriteString(fmt.Sprintf("[v%d]", i+1))
	}
	filterComplex.WriteString("; ")

	for i, res := range Resolutions {
		filterComplex.WriteString(fmt.Sprintf("[v%d]scale=w=%d:h=%d[v%dout]", i+1, res.Width, res.Height, i+1))
		if i < len(Resolutions)-1 {
			filterComplex.WriteString("; ")
		}
	}

	args = append(args, "-filter_complex", filterComplex.String())

	for i, res := range Resolutions {
		args = append(args,
			"-map", fmt.Sprintf("[v%dout]", i+1),
			"-map", "0:a?",
			fmt.Sprintf("-c:v:%d", i), "libx264",
			fmt.Sprintf("-b:v:%d", i), getVideoBitrate(res.Height),
			fmt.Sprintf("-c:a:%d", i), "aac",
			fmt.Sprintf("-b:a:%d", i), "128k",
		)
	}

	args = append(args,
		"-f", "hls",
		"-hls_time", "4",
		"-hls_playlist_type", "vod",
		"-hls_flags", "independent_segments",
		"-hls_segment_type", "mpegts",
		"-master_pl_name", "master.m3u8",
		"-var_stream_map", "v:0,a:0 v:1,a:1 v:2,a:2 v:3,a:3",
		filepath.Join(outputDir, "stream_%v.m3u8"),
	)

	log.Printf("Running HLS ffmpeg command: ffmpeg %s", strings.Join(args, " "))

	cmd := exec.Command("ffmpeg", args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	return cmd.Run()
}

func (s *PackagerService) packageDASH(inputPath, outputDir string) error {
	args := []string{
		"-i", inputPath,
		"-y",
	}

	var filterComplex strings.Builder
	filterComplex.WriteString("[0:v]split=4")
	for i := range Resolutions {
		filterComplex.WriteString(fmt.Sprintf("[v%d]", i+1))
	}
	filterComplex.WriteString("; ")

	for i, res := range Resolutions {
		filterComplex.WriteString(fmt.Sprintf("[v%d]scale=w=%d:h=%d[v%dout]", i+1, res.Width, res.Height, i+1))
		if i < len(Resolutions)-1 {
			filterComplex.WriteString("; ")
		}
	}

	args = append(args, "-filter_complex", filterComplex.String())

	for i, res := range Resolutions {
		args = append(args,
			"-map", fmt.Sprintf("[v%dout]", i+1),
			"-map", "0:a?",
			fmt.Sprintf("-c:v:%d", i), "libx264",
			fmt.Sprintf("-b:v:%d", i), getVideoBitrate(res.Height),
			fmt.Sprintf("-c:a:%d", i), "aac",
			fmt.Sprintf("-b:a:%d", i), "128k",
		)
	}

	args = append(args,
		"-f", "dash",
		"-seg_duration", "4",
		"-use_timeline", "1",
		"-use_template", "1",
		"-adaptation_sets", "id=0,streams=v id=1,streams=a",
		filepath.Join(outputDir, "manifest.mpd"),
	)

	log.Printf("Running DASH ffmpeg command: ffmpeg %s", strings.Join(args, " "))

	cmd := exec.Command("ffmpeg", args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	return cmd.Run()
}

func getVideoBitrate(height int) string {
	switch height {
	case 360:
		return "800k"
	case 480:
		return "1400k"
	case 720:
		return "2800k"
	case 1080:
		return "5000k"
	default:
		return "2000k"
	}
}
