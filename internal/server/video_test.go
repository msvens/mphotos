package server

import (
	"errors"
	"fmt"
	"testing"

	"github.com/msvens/mimage/video"
)

// TestVideoErrorCategory maps mimage's typed transcode errors (including wrapped
// ones) to the stable categories stored in import_error and shown in job failures.
func TestVideoErrorCategory(t *testing.T) {
	cases := []struct {
		name string
		err  error
		want string
	}{
		{"hdr", video.ErrHDRUnsupported, "hdr"},
		{"hdr wrapped", fmt.Errorf("transcode: %w", video.ErrHDRUnsupported), "hdr"},
		{"truncated", video.ErrTruncated, "truncated"},
		{"no video stream", video.ErrNoVideoStream, "no-video-stream"},
		{"not video", video.ErrNotVideo, "not-video"},
		{"tool error", &video.ToolError{Tool: "ffmpeg", Stderr: "boom", Err: errors.New("exit 1")}, "tool-error"},
		{"other", errors.New("something else"), "error"},
	}
	for _, c := range cases {
		if got := videoErrorCategory(c.err); got != c.want {
			t.Errorf("%s: videoErrorCategory = %q, want %q", c.name, got, c.want)
		}
	}
}

// TestParseDateFromName covers the filename capture-date fallback used when a video
// carries no creation time (e.g. Samsung VID_YYYYMMDD_HHMMSS).
func TestParseDateFromName(t *testing.T) {
	tm, ok := parseDateFromName("VID_20260115_143005.mp4")
	if !ok {
		t.Fatal("expected a date to parse from VID_20260115_143005.mp4")
	}
	if tm.Year() != 2026 || tm.Month() != 1 || tm.Day() != 15 ||
		tm.Hour() != 14 || tm.Minute() != 30 || tm.Second() != 5 {
		t.Errorf("wrong parsed date: %v", tm)
	}
	if _, ok := parseDateFromName("random_clip.mp4"); ok {
		t.Error("expected no date from a name without a timestamp")
	}
}
