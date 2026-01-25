# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

This is a video packager that converts MP4 video files into HLS and DASH streaming formats.

**Input:** Video files from `./.videos/` directory
**Output:** Packaged streams saved to `./.playlists/{video_name}/[hls|dash]/`
**Supported resolutions:** 360p, 480p, 720p, 1080p

## Build and Run Commands

```bash
# Build the project
go build

# Run the packager
go run .

# Run tests
go test ./...

# Run a single test
go test -run TestName ./path/to/package
```

## Project Structure

- `.videos/` - Input directory for source MP4 files
- `.playlists/` - Output directory for packaged HLS/DASH streams
- `.docs/` - Project documentation
