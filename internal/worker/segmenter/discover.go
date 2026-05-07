package segmenter

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"hvc/internal/model"
	"hvc/internal/worker/planner"
)

var (
	initSegRe  = regexp.MustCompile(`^init-(.+)-(\d+)\.m4s$`)
	mediaSegRe = regexp.MustCompile(`^seg-(.+)-(\d+)-(\d+)\.m4s$`)
)

type DiscoveredSegment struct {
	RenditionName string
	IsInit        bool
	SequenceNo    int
	RepresentationID int
	ObjectKey     string
	FilePath      string
	FileSize      int64
	MediaType     int
}

type Result struct {
	Segments []DiscoveredSegment
}

func Discover(job model.TranscodeJob, pipeline planner.Pipeline) Result {
	dir := pipeline.OutputDir
	entries, err := os.ReadDir(dir)
	if err != nil {
		return Result{}
	}

	var segments []DiscoveredSegment
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		info, err := entry.Info()
		if err != nil {
			continue
		}

		if m := initSegRe.FindStringSubmatch(name); m != nil {
			renditionName := m[1]
			repID, _ := strconv.Atoi(m[2])
			segments = append(segments, DiscoveredSegment{
				RenditionName:    renditionName,
				IsInit:           true,
				RepresentationID: repID,
				ObjectKey:        buildObjectKey(job, renditionName, name),
				FilePath:         filepath.Join(dir, name),
				FileSize:         info.Size(),
				MediaType:        0,
			})
		} else if m := mediaSegRe.FindStringSubmatch(name); m != nil {
			renditionName := m[1]
			repID, _ := strconv.Atoi(m[2])
			seqNo, _ := strconv.Atoi(m[3])
			mediaType := 1
			if strings.Contains(renditionName, "audio") {
				mediaType = 2
			}
			segments = append(segments, DiscoveredSegment{
				RenditionName:    renditionName,
				IsInit:           false,
				SequenceNo:       seqNo,
				RepresentationID: repID,
				ObjectKey:        buildObjectKey(job, renditionName, name),
				FilePath:         filepath.Join(dir, name),
				FileSize:         info.Size(),
				MediaType:        mediaType,
			})
		}
	}

	sort.Slice(segments, func(i, j int) bool {
		if segments[i].RenditionName != segments[j].RenditionName {
			return segments[i].RenditionName < segments[j].RenditionName
		}
		if segments[i].IsInit != segments[j].IsInit {
			return segments[i].IsInit
		}
		return segments[i].SequenceNo < segments[j].SequenceNo
	})

	return Result{Segments: segments}
}

func buildObjectKey(job model.TranscodeJob, renditionName string, fileName string) string {
	prefix := job.OutputBasePrefix
	if prefix == "" {
		prefix = fmt.Sprintf("transcode/%d", job.JobID)
	}
	return fmt.Sprintf("%s/%s/%s", prefix, renditionName, fileName)
}
