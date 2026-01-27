package services

import (
	"bytes"
	"fmt"
	"strings"
	"text/template"
)

type ManifestService struct {
	segmentDuration int
}

type VideoParams struct {
	Codec     string
	Width     uint32
	Height    uint32
	Timescale uint32
}

func NewManifestService(segmentDurationSec int) *ManifestService {
	return &ManifestService{
		segmentDuration: segmentDurationSec,
	}
}

// HLS Master Playlist
func (m *ManifestService) GenerateHLSMasterPlaylist(videoName string, durationSec float64, params VideoParams) string {
	var buf bytes.Buffer
	buf.WriteString("#EXTM3U\n")
	buf.WriteString("#EXT-X-VERSION:6\n")
	buf.WriteString("\n")

	codec := params.Codec
	if codec == "" {
		codec = "avc1.640028"
	}

	// Single quality (original) - video only
	buf.WriteString(fmt.Sprintf("#EXT-X-STREAM-INF:BANDWIDTH=5000000,RESOLUTION=%dx%d,CODECS=\"%s\"\n",
		params.Width, params.Height, codec))
	buf.WriteString("media.m3u8\n")

	return buf.String()
}

// HLS Media Playlist
func (m *ManifestService) GenerateHLSMediaPlaylist(videoName string, durationSec float64, segmentCount int) string {
	var buf bytes.Buffer
	buf.WriteString("#EXTM3U\n")
	buf.WriteString("#EXT-X-VERSION:6\n")
	buf.WriteString(fmt.Sprintf("#EXT-X-TARGETDURATION:%d\n", m.segmentDuration))
	buf.WriteString("#EXT-X-MEDIA-SEQUENCE:0\n")
	buf.WriteString("#EXT-X-PLAYLIST-TYPE:VOD\n")
	buf.WriteString("#EXT-X-MAP:URI=\"init.mp4\"\n")
	buf.WriteString("\n")

	remainingDuration := durationSec
	for i := 0; i < segmentCount; i++ {
		segDur := float64(m.segmentDuration)
		if remainingDuration < segDur {
			segDur = remainingDuration
		}
		buf.WriteString(fmt.Sprintf("#EXTINF:%.6f,\n", segDur))
		buf.WriteString(fmt.Sprintf("segment_%d.m4s\n", i))
		remainingDuration -= segDur
	}

	buf.WriteString("#EXT-X-ENDLIST\n")
	return buf.String()
}

// DASH MPD Template - video only
const dashMPDTemplate = `<?xml version="1.0" encoding="UTF-8"?>
<MPD xmlns="urn:mpeg:dash:schema:mpd:2011"
     xmlns:xsi="http://www.w3.org/2001/XMLSchema-instance"
     xsi:schemaLocation="urn:mpeg:dash:schema:mpd:2011 DASH-MPD.xsd"
     type="static"
     mediaPresentationDuration="PT{{.DurationStr}}"
     minBufferTime="PT2S"
     profiles="urn:mpeg:dash:profile:isoff-on-demand:2011">
  <Period id="0" start="PT0S">
    <AdaptationSet id="0" contentType="video" mimeType="video/mp4" segmentAlignment="true" bitstreamSwitching="true">
      <Representation id="video" codecs="{{.Codec}}"
                      bandwidth="5000000" width="{{.Width}}" height="{{.Height}}">
        <SegmentTemplate timescale="{{.Timescale}}"
                         initialization="init.mp4"
                         media="segment_$Number$.m4s"
                         startNumber="0">
          <SegmentTimeline>
{{.SegmentTimeline}}
          </SegmentTimeline>
        </SegmentTemplate>
      </Representation>
    </AdaptationSet>
  </Period>
</MPD>`

type DASHMPDData struct {
	DurationStr     string
	Timescale       uint32
	SegmentTimeline string
	Codec           string
	Width           uint32
	Height          uint32
}

func (m *ManifestService) GenerateDASHMPD(videoName string, durationSec float64, segmentCount int, params VideoParams) (string, error) {
	// Format duration as ISO 8601 duration
	hours := int(durationSec) / 3600
	minutes := (int(durationSec) % 3600) / 60
	seconds := durationSec - float64(hours*3600+minutes*60)

	var durationStr string
	if hours > 0 {
		durationStr = fmt.Sprintf("%dH%dM%.3fS", hours, minutes, seconds)
	} else if minutes > 0 {
		durationStr = fmt.Sprintf("%dM%.3fS", minutes, seconds)
	} else {
		durationStr = fmt.Sprintf("%.3fS", seconds)
	}

	// Generate segment timeline
	var timeline strings.Builder
	segmentDurTS := uint64(m.segmentDuration) * uint64(params.Timescale)
	totalDurTS := uint64(durationSec * float64(params.Timescale))

	var currentTime uint64 = 0
	for i := 0; i < segmentCount; i++ {
		dur := segmentDurTS
		if currentTime+dur > totalDurTS {
			dur = totalDurTS - currentTime
		}
		if i == 0 {
			timeline.WriteString(fmt.Sprintf("            <S t=\"0\" d=\"%d\"/>\n", dur))
		} else {
			timeline.WriteString(fmt.Sprintf("            <S d=\"%d\"/>\n", dur))
		}
		currentTime += dur
	}

	codec := params.Codec
	if codec == "" {
		codec = "avc1.640028"
	}

	data := DASHMPDData{
		DurationStr:     durationStr,
		Timescale:       params.Timescale,
		SegmentTimeline: strings.TrimSuffix(timeline.String(), "\n"),
		Codec:           codec,
		Width:           params.Width,
		Height:          params.Height,
	}

	tmpl, err := template.New("mpd").Parse(dashMPDTemplate)
	if err != nil {
		return "", err
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return "", err
	}

	return buf.String(), nil
}
