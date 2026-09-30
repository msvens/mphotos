package server

import "net/http"

// Capabilities advertises server-side features the frontend needs to know about,
// independent of any Drive connection or login. Extend as more capabilities appear.
type Capabilities struct {
	// VideoEnabled is true when ffmpeg/ffprobe are available (video import/upload works).
	VideoEnabled bool `json:"videoEnabled"`
}

// handleCapabilities reports server capabilities. Public and Drive-independent, so
// a local-only setup can still discover that video is available.
func (s *mserver) handleCapabilities(_ *http.Request, _ bool) (interface{}, error) {
	return Capabilities{VideoEnabled: s.videoEnabled}, nil
}
