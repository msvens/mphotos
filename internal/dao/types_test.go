package dao

import (
	"encoding/json"
	"testing"

	"github.com/google/uuid"
)

// TestGuestJSONExposesGuestId guards the fix that lets a logged-in guest build
// their own avatar URL (GET /api/guest/avatar/{guestId}): the Guest record must
// serialize its id as "guestId", matching the name reactions/comments already
// use. It also pins that the internal googleId stays hidden. Pure marshal, no DB.
func TestGuestJSONExposesGuestId(t *testing.T) {
	id := uuid.New()
	b, err := json.Marshal(Guest{Id: id, Email: "g@example.com", GoogleId: "secret"})
	if err != nil {
		t.Fatalf("marshal Guest: %v", err)
	}
	var m map[string]any
	if err := json.Unmarshal(b, &m); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if m["guestId"] != id.String() {
		t.Errorf("guestId = %v, want %s", m["guestId"], id)
	}
	if _, ok := m["email"]; !ok {
		t.Error("email should be present in the guest's own record")
	}
	if _, ok := m["googleId"]; ok {
		t.Error("googleId must never be serialized")
	}
}

// TestPhotoSourceOtherJSON: the original image format is exposed as "sourceOther"
// when set and omitted when empty (photos imported before conversion tracking).
// The internal sourceId stays hidden.
func TestPhotoSourceOtherJSON(t *testing.T) {
	b, err := json.Marshal(Photo{SourceOther: "tiff", SourceId: "drive-123"})
	if err != nil {
		t.Fatalf("marshal Photo: %v", err)
	}
	var m map[string]any
	if err := json.Unmarshal(b, &m); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if m["sourceOther"] != "tiff" {
		t.Errorf("sourceOther = %v, want tiff", m["sourceOther"])
	}
	if _, ok := m["sourceId"]; ok {
		t.Error("sourceId must never be serialized")
	}

	b, _ = json.Marshal(Photo{})
	m = map[string]any{}
	if err := json.Unmarshal(b, &m); err != nil {
		t.Fatalf("unmarshal empty: %v", err)
	}
	if _, ok := m["sourceOther"]; ok {
		t.Error("empty sourceOther should be omitted (omitempty)")
	}
}
