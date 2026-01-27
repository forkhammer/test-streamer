package services

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"sync"

	"github.com/Eyevinn/mp4ff/mp4"
)

type Segmenter struct {
	segmentDuration uint64 // in seconds
	mu              sync.RWMutex
	videoCache      map[string]*VideoFile
}

type VideoFile struct {
	Path       string
	File       *os.File
	MP4        *mp4.File
	Timescale  uint32
	Duration   uint64
	VideoTrack *mp4.TrakBox
	AudioTrack *mp4.TrakBox
	VideoCodec string
	Width      uint32
	Height     uint32
}

func NewSegmenter(segmentDurationSec int) *Segmenter {
	return &Segmenter{
		segmentDuration: uint64(segmentDurationSec),
		videoCache:      make(map[string]*VideoFile),
	}
}

func (s *Segmenter) OpenVideo(path string) (*VideoFile, error) {
	s.mu.RLock()
	if vf, ok := s.videoCache[path]; ok {
		s.mu.RUnlock()
		return vf, nil
	}
	s.mu.RUnlock()

	s.mu.Lock()
	defer s.mu.Unlock()

	// Double check after acquiring write lock
	if vf, ok := s.videoCache[path]; ok {
		return vf, nil
	}

	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("failed to open video file: %w", err)
	}

	parsedFile, err := mp4.DecodeFile(f)
	if err != nil {
		f.Close()
		return nil, fmt.Errorf("failed to parse MP4: %w", err)
	}

	if parsedFile.Moov == nil {
		f.Close()
		return nil, fmt.Errorf("no moov box found in MP4")
	}

	vf := &VideoFile{
		Path: path,
		File: f,
		MP4:  parsedFile,
	}

	// Find video and audio tracks
	for _, trak := range parsedFile.Moov.Traks {
		if trak.Mdia == nil || trak.Mdia.Hdlr == nil {
			continue
		}

		switch trak.Mdia.Hdlr.HandlerType {
		case "vide":
			vf.VideoTrack = trak
			if trak.Mdia.Mdhd != nil {
				vf.Timescale = trak.Mdia.Mdhd.Timescale
				vf.Duration = trak.Mdia.Mdhd.Duration
			}
			// Extract video codec and dimensions
			if trak.Tkhd != nil {
				vf.Width = uint32(trak.Tkhd.Width >> 16)
				vf.Height = uint32(trak.Tkhd.Height >> 16)
			}
			vf.VideoCodec = extractVideoCodec(trak)
		case "soun":
			vf.AudioTrack = trak
		}
	}

	if vf.VideoTrack == nil {
		f.Close()
		return nil, fmt.Errorf("no video track found")
	}

	s.videoCache[path] = vf
	return vf, nil
}

func extractVideoCodec(trak *mp4.TrakBox) string {
	if trak.Mdia == nil || trak.Mdia.Minf == nil || trak.Mdia.Minf.Stbl == nil || trak.Mdia.Minf.Stbl.Stsd == nil {
		return "avc1.640028" // Default fallback
	}

	stsd := trak.Mdia.Minf.Stbl.Stsd

	// Check for AVC (H.264)
	if stsd.AvcX != nil {
		if stsd.AvcX.AvcC != nil {
			avcC := stsd.AvcX.AvcC
			// Build codec string like "avc1.64001f"
			if len(avcC.SPSnalus) > 0 && len(avcC.SPSnalus[0]) >= 4 {
				sps := avcC.SPSnalus[0]
				return fmt.Sprintf("avc1.%02x%02x%02x", sps[1], sps[2], sps[3])
			}
			return fmt.Sprintf("avc1.%02x%02x%02x", avcC.AVCProfileIndication, avcC.ProfileCompatibility, avcC.AVCLevelIndication)
		}
		return "avc1.640028"
	}

	// Check for HEVC (H.265)
	if stsd.HvcX != nil {
		return "hvc1.1.6.L93.B0" // Common HEVC codec string
	}

	return "avc1.640028" // Default fallback
}

func (s *Segmenter) GetSegmentCount(vf *VideoFile) int {
	if vf.Timescale == 0 {
		return 0
	}
	segmentDurTS := s.segmentDuration * uint64(vf.Timescale)
	return int((vf.Duration + segmentDurTS - 1) / segmentDurTS)
}

func (s *Segmenter) GetDurationSec(vf *VideoFile) float64 {
	if vf.Timescale == 0 {
		return 0
	}
	return float64(vf.Duration) / float64(vf.Timescale)
}

func (s *Segmenter) GenerateInitSegment(vf *VideoFile) ([]byte, error) {
	buf := &bytes.Buffer{}

	// Create ftyp box
	ftyp := mp4.NewFtyp("isom", 0x200, []string{"isom", "iso2", "avc1", "mp41"})
	if err := ftyp.Encode(buf); err != nil {
		return nil, fmt.Errorf("failed to encode ftyp: %w", err)
	}

	// Create moov for fragmented MP4
	moov := mp4.NewMoovBox()

	// Add mvhd
	mvhd := mp4.CreateMvhd()
	mvhd.Timescale = vf.Timescale
	mvhd.Duration = 0 // For fragmented MP4, duration in moov should be 0
	mvhd.NextTrackID = 2
	moov.AddChild(mvhd)

	// Add video track only (no audio to avoid codec mismatch issues)
	if vf.VideoTrack != nil {
		videoTrak := s.createFragmentedTrak(vf.VideoTrack, 1)
		moov.AddChild(videoTrak)
	}

	// Add mvex for fragmented MP4
	mvex := &mp4.MvexBox{}
	if vf.VideoTrack != nil {
		trex := mp4.CreateTrex(1)
		trex.DefaultSampleDescriptionIndex = 1
		trex.DefaultSampleDuration = 0
		trex.DefaultSampleSize = 0
		trex.DefaultSampleFlags = 0
		mvex.AddChild(trex)
	}
	moov.AddChild(mvex)

	if err := moov.Encode(buf); err != nil {
		return nil, fmt.Errorf("failed to encode moov: %w", err)
	}

	return buf.Bytes(), nil
}

func (s *Segmenter) createFragmentedTrak(srcTrak *mp4.TrakBox, trackID uint32) *mp4.TrakBox {
	trak := &mp4.TrakBox{}

	// Create tkhd
	tkhd := mp4.CreateTkhd()
	tkhd.Version = 0
	tkhd.Flags = 0x000003 // Track enabled, in movie
	tkhd.TrackID = trackID
	tkhd.Duration = 0 // Will be in fragments
	tkhd.Width = srcTrak.Tkhd.Width
	tkhd.Height = srcTrak.Tkhd.Height
	tkhd.Volume = srcTrak.Tkhd.Volume
	tkhd.AlternateGroup = srcTrak.Tkhd.AlternateGroup
	tkhd.Layer = srcTrak.Tkhd.Layer
	trak.AddChild(tkhd)

	// Create mdia
	mdia := &mp4.MdiaBox{}

	// Create mdhd
	mdhd := &mp4.MdhdBox{
		Version:   0,
		Timescale: srcTrak.Mdia.Mdhd.Timescale,
		Duration:  0,
		Language:  srcTrak.Mdia.Mdhd.Language,
	}
	mdia.AddChild(mdhd)

	// Copy hdlr
	hdlr := &mp4.HdlrBox{
		HandlerType: srcTrak.Mdia.Hdlr.HandlerType,
		Name:        srcTrak.Mdia.Hdlr.Name,
	}
	mdia.AddChild(hdlr)

	// Create minf
	minf := &mp4.MinfBox{}

	// Add appropriate header based on track type
	if srcTrak.Mdia.Hdlr.HandlerType == "vide" {
		vmhd := &mp4.VmhdBox{
			Flags: 1,
		}
		minf.AddChild(vmhd)
	} else if srcTrak.Mdia.Hdlr.HandlerType == "soun" {
		smhd := &mp4.SmhdBox{}
		minf.AddChild(smhd)
	}

	// Create dinf
	dinf := &mp4.DinfBox{}
	dref := &mp4.DrefBox{}
	url := &mp4.URLBox{Flags: 1} // Self-contained
	dref.AddChild(url)
	dinf.AddChild(dref)
	minf.AddChild(dinf)

	// Copy stbl (sample table) - needed for codec info
	if srcTrak.Mdia.Minf != nil && srcTrak.Mdia.Minf.Stbl != nil {
		stbl := s.createEmptyStbl(srcTrak.Mdia.Minf.Stbl)
		minf.AddChild(stbl)
	}

	mdia.AddChild(minf)
	trak.AddChild(mdia)

	return trak
}

func (s *Segmenter) createEmptyStbl(srcStbl *mp4.StblBox) *mp4.StblBox {
	stbl := &mp4.StblBox{}

	// Copy stsd (sample description - codec info)
	if srcStbl.Stsd != nil {
		stbl.AddChild(srcStbl.Stsd)
	}

	// Empty stts
	stts := &mp4.SttsBox{}
	stbl.AddChild(stts)

	// Empty stsc
	stsc := &mp4.StscBox{}
	stbl.AddChild(stsc)

	// Empty stsz
	stsz := &mp4.StszBox{}
	stbl.AddChild(stsz)

	// Empty stco
	stco := &mp4.StcoBox{}
	stbl.AddChild(stco)

	return stbl
}

func (s *Segmenter) GenerateMediaSegment(vf *VideoFile, segmentIndex int) ([]byte, error) {
	if vf.VideoTrack == nil {
		return nil, fmt.Errorf("no video track")
	}

	stbl := vf.VideoTrack.Mdia.Minf.Stbl
	if stbl == nil {
		return nil, fmt.Errorf("no stbl box")
	}

	segmentDurTS := s.segmentDuration * uint64(vf.Timescale)
	startTime := uint64(segmentIndex) * segmentDurTS
	endTime := startTime + segmentDurTS
	if endTime > vf.Duration {
		endTime = vf.Duration
	}

	// Get sample range for this segment
	startSample, endSample, err := s.getSampleRange(stbl, startTime, endTime)
	if err != nil {
		return nil, err
	}

	if startSample >= endSample {
		return nil, fmt.Errorf("no samples in segment %d", segmentIndex)
	}

	buf := &bytes.Buffer{}

	// Create styp box
	styp := mp4.NewStyp("msdh", 0, []string{"msdh", "msix"})
	if err := styp.Encode(buf); err != nil {
		return nil, err
	}
	stypSize := buf.Len()

	// Create traf and get sample data
	traf, mdatData, err := s.createTraf(vf, stbl, 1, startSample, endSample, startTime)
	if err != nil {
		return nil, err
	}

	// Create moof box
	moof := &mp4.MoofBox{}

	// Add mfhd
	mfhd := &mp4.MfhdBox{
		SequenceNumber: uint32(segmentIndex + 1),
	}
	moof.AddChild(mfhd)
	moof.AddChild(traf)

	// Calculate moof size to determine data offset
	moofSize := moof.Size()

	// mdat header size (8 bytes for regular, 16 for extended)
	mdatHeaderSize := 8
	if len(mdatData)+8 > 0xFFFFFFFF {
		mdatHeaderSize = 16
	}

	// DataOffset is relative to the start of moof, pointing to mdat payload
	// DataOffset = moofSize + mdatHeaderSize (from moof start to mdat data)
	dataOffset := int32(moofSize) + int32(mdatHeaderSize)

	// Update trun DataOffset
	if traf.Trun != nil {
		traf.Trun.DataOffset = dataOffset
	}

	if err := moof.Encode(buf); err != nil {
		return nil, err
	}

	// Verify moof was written correctly
	actualMoofSize := buf.Len() - stypSize
	if uint64(actualMoofSize) != moofSize {
		// Recalculate if size changed
		dataOffset = int32(actualMoofSize) + int32(mdatHeaderSize)
		// Need to re-encode - but for now just use the calculated value
	}

	// Create mdat box
	mdat := &mp4.MdatBox{
		Data: mdatData,
	}
	if err := mdat.Encode(buf); err != nil {
		return nil, err
	}

	return buf.Bytes(), nil
}

func (s *Segmenter) getSampleRange(stbl *mp4.StblBox, startTime, endTime uint64) (uint32, uint32, error) {
	stts := stbl.Stts
	if stts == nil {
		return 0, 0, fmt.Errorf("no stts box")
	}

	var sampleNum uint32 = 1
	var currentTime uint64 = 0
	var startSample, endSample uint32 = 0, 0

	// If starting from 0, first sample is always 1
	if startTime == 0 {
		startSample = 1
	}

	for i, count := range stts.SampleCount {
		delta := stts.SampleTimeDelta[i]
		for j := uint32(0); j < count; j++ {
			if startSample == 0 && currentTime >= startTime {
				startSample = sampleNum
			}
			currentTime += uint64(delta)
			if currentTime >= endTime {
				endSample = sampleNum + 1
				return startSample, endSample, nil
			}
			sampleNum++
		}
	}

	endSample = sampleNum
	if startSample == 0 {
		startSample = 1
	}
	return startSample, endSample, nil
}

func (s *Segmenter) createTraf(vf *VideoFile, stbl *mp4.StblBox, trackID uint32, startSample, endSample uint32, baseTime uint64) (*mp4.TrafBox, []byte, error) {
	traf := &mp4.TrafBox{}

	// Create tfhd using factory function
	tfhd := mp4.CreateTfhd(trackID)
	tfhd.Flags = 0x020000 // Default-base-is-moof
	traf.AddChild(tfhd)

	// Create tfdt using factory function
	tfdt := mp4.CreateTfdt(baseTime)
	traf.AddChild(tfdt)

	// Create trun and read sample data
	trun := &mp4.TrunBox{
		Version: 0,
		Flags:   0x000F01, // Data offset, duration, size, flags, composition time offset
	}

	var mdatData bytes.Buffer

	// Get sample sizes
	stsz := stbl.Stsz
	if stsz == nil {
		return nil, nil, fmt.Errorf("no stsz box")
	}

	// Get sample durations from stts
	stts := stbl.Stts
	durations := s.expandSampleDurations(stts, endSample)

	// Get sync samples (keyframes)
	var syncSamples map[uint32]bool
	if stbl.Stss != nil {
		syncSamples = make(map[uint32]bool)
		for _, ss := range stbl.Stss.SampleNumber {
			syncSamples[ss] = true
		}
	}

	// Get composition time offsets
	ctts := stbl.Ctts

	for sampleNum := startSample; sampleNum < endSample; sampleNum++ {
		// Get sample size using the GetSampleSize method (1-indexed)
		size := stsz.GetSampleSize(int(sampleNum))

		// Get sample duration
		var duration uint32 = 1024 // Default
		if int(sampleNum-1) < len(durations) {
			duration = durations[sampleNum-1]
		}

		// Get sample flags
		var flags uint32 = 0x1010000 // Non-sync sample
		if syncSamples == nil || syncSamples[sampleNum] {
			flags = 0x2000000 // Sync sample (keyframe)
		}

		// Get composition time offset
		var cto int32 = 0
		if ctts != nil {
			cto = ctts.GetCompositionTimeOffset(sampleNum)
		}

		sample := mp4.NewSample(flags, duration, size, cto)
		trun.Samples = append(trun.Samples, sample)

		// Read sample data
		offset, err := s.getSampleOffset(stbl, sampleNum)
		if err != nil {
			return nil, nil, err
		}

		sampleData := make([]byte, size)
		if _, err := vf.File.Seek(int64(offset), io.SeekStart); err != nil {
			return nil, nil, err
		}
		if _, err := io.ReadFull(vf.File, sampleData); err != nil {
			return nil, nil, err
		}

		mdatData.Write(sampleData)
	}

	// DataOffset will be set later, but set a placeholder so Size() includes it
	trun.DataOffset = 1
	traf.AddChild(trun)

	return traf, mdatData.Bytes(), nil
}

func (s *Segmenter) expandSampleDurations(stts *mp4.SttsBox, maxSample uint32) []uint32 {
	result := make([]uint32, 0, maxSample)
	for i, count := range stts.SampleCount {
		dur := stts.SampleTimeDelta[i]
		for j := uint32(0); j < count && uint32(len(result)) < maxSample; j++ {
			result = append(result, dur)
		}
	}
	return result
}

func (s *Segmenter) getSampleOffset(stbl *mp4.StblBox, sampleNum uint32) (uint64, error) {
	stsc := stbl.Stsc
	stco := stbl.Stco
	co64 := stbl.Co64
	stsz := stbl.Stsz

	if stsc == nil {
		return 0, fmt.Errorf("no stsc box")
	}
	if stco == nil && co64 == nil {
		return 0, fmt.Errorf("no stco or co64 box")
	}

	// Use StscBox method to find chunk for sample
	chunkNr, firstSampleInChunk, err := stsc.ChunkNrFromSampleNr(int(sampleNum))
	if err != nil {
		return 0, fmt.Errorf("failed to find chunk for sample %d: %w", sampleNum, err)
	}

	// Get chunk offset
	var chunkOffset uint64
	if stco != nil && chunkNr-1 < len(stco.ChunkOffset) {
		chunkOffset = uint64(stco.ChunkOffset[chunkNr-1])
	} else if co64 != nil && chunkNr-1 < len(co64.ChunkOffset) {
		chunkOffset = co64.ChunkOffset[chunkNr-1]
	} else {
		return 0, fmt.Errorf("chunk %d offset not found", chunkNr)
	}

	// Calculate offset within chunk
	var offsetInChunk uint64 = 0
	for i := firstSampleInChunk; i < int(sampleNum); i++ {
		size := stsz.GetSampleSize(i) // GetSampleSize is 1-indexed
		offsetInChunk += uint64(size)
	}

	return chunkOffset + offsetInChunk, nil
}

func (s *Segmenter) Close() {
	s.mu.Lock()
	defer s.mu.Unlock()

	for _, vf := range s.videoCache {
		if vf.File != nil {
			vf.File.Close()
		}
	}
	s.videoCache = make(map[string]*VideoFile)
}
