package dao

import (
	"bytes"
	"testing"
	"time"

	"github.com/google/uuid"
)

// mkSelectPhoto builds a Photo with all NOT NULL columns populated, varying only
// the fields the Select tests care about (camera model, upload and original date).
func mkSelectPhoto(model string, upload, original time.Time) Photo {
	return Photo{
		Id:           uuid.New(),
		Md5:          uuid.New().String(),
		Source:       "local",
		UploadDate:   upload,
		OriginalDate: original,
		FileName:     model + ".jpg",
		Title:        model,
		CameraMake:   "TestMake",
		CameraModel:  model,
		Iso:          100,
		FNumber:      2.8,
		Exposure:     "1/100",
		Width:        1000,
		Height:       800,
	}
}

func idsOf(photos []*Photo) []uuid.UUID {
	ids := make([]uuid.UUID, len(photos))
	for i, p := range photos {
		ids[i] = p.Id
	}
	return ids
}

func TestPhotoSelect(t *testing.T) {
	pgdb := openAndCreateTestDb(t)
	defer deleteAndCloseTestDb(pgdb, t)

	base := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)
	// Upload dates: pA > pB > (pC == pD). The pC/pD tie exercises the id
	// tie-breaker. Original dates are all distinct and in a different order than
	// upload dates, so an OriginalDate sort must differ from an UploadDate sort.
	pA := mkSelectPhoto("Alpha", base.Add(3*time.Hour), base.Add(1*time.Hour))
	pB := mkSelectPhoto("Beta", base.Add(2*time.Hour), base.Add(4*time.Hour))
	pC := mkSelectPhoto("Alpha", base.Add(1*time.Hour), base.Add(2*time.Hour))
	pD := mkSelectPhoto("Gamma", base.Add(1*time.Hour), base.Add(3*time.Hour))

	for _, p := range []Photo{pA, pB, pC, pD} {
		pc := p
		if err := pgdb.Photo.Add(&pc, nil); err != nil {
			t.Fatalf("could not add photo %s: %v", p.Title, err)
		}
	}

	// the tie pair (pC, pD) ordered by id ascending, matching Postgres UUID order
	tieLo, tieHi := pC, pD
	if bytes.Compare(pC.Id[:], pD.Id[:]) > 0 {
		tieLo, tieHi = pD, pC
	}

	// 1. No filter, no paging: all photos, uploaddate DESC with id tie-breaker.
	if photos, err := pgdb.Photo.Select(PhotoFilter{}, Range{}, UploadDate); err != nil {
		t.Fatalf("Select all failed: %v", err)
	} else {
		want := []uuid.UUID{pA.Id, pB.Id, tieLo.Id, tieHi.Id}
		if got := idsOf(photos); !equalIDs(got, want) {
			t.Errorf("uploaddate order: expected %v got %v", want, got)
		}
	}

	// 2. None order falls back to uploaddate DESC (historic List behavior).
	if photos, err := pgdb.Photo.Select(PhotoFilter{}, Range{}, None); err != nil {
		t.Fatalf("Select None failed: %v", err)
	} else {
		want := []uuid.UUID{pA.Id, pB.Id, tieLo.Id, tieHi.Id}
		if got := idsOf(photos); !equalIDs(got, want) {
			t.Errorf("None order: expected uploaddate fallback %v got %v", want, got)
		}
	}

	// 3. OriginalDate order is distinct and total.
	if photos, err := pgdb.Photo.Select(PhotoFilter{}, Range{}, OriginalDate); err != nil {
		t.Fatalf("Select OriginalDate failed: %v", err)
	} else {
		want := []uuid.UUID{pB.Id, pD.Id, pC.Id, pA.Id}
		if got := idsOf(photos); !equalIDs(got, want) {
			t.Errorf("originaldate order: expected %v got %v", want, got)
		}
	}

	// 4. Paging is stable across the tie: two disjoint pages cover all rows with
	// no duplicate or dropped row.
	page1, err := pgdb.Photo.Select(PhotoFilter{}, Range{Limit: 2, Offset: 0}, UploadDate)
	if err != nil {
		t.Fatalf("page1 failed: %v", err)
	}
	page2, err := pgdb.Photo.Select(PhotoFilter{}, Range{Limit: 2, Offset: 2}, UploadDate)
	if err != nil {
		t.Fatalf("page2 failed: %v", err)
	}
	if len(page1) != 2 || len(page2) != 2 {
		t.Fatalf("expected two pages of 2, got %d and %d", len(page1), len(page2))
	}
	seen := map[uuid.UUID]bool{}
	for _, id := range append(idsOf(page1), idsOf(page2)...) {
		if seen[id] {
			t.Errorf("paging returned duplicate id %v", id)
		}
		seen[id] = true
	}
	if len(seen) != 4 {
		t.Errorf("paging did not cover all 4 photos, got %d distinct", len(seen))
	}

	// 5. Limit == 0 means no limit — all rows.
	if photos, err := pgdb.Photo.Select(PhotoFilter{}, Range{Limit: 0, Offset: 0}, UploadDate); err != nil {
		t.Fatalf("Select limit 0 failed: %v", err)
	} else if len(photos) != 4 {
		t.Errorf("limit 0 should return all 4, got %d", len(photos))
	}

	// 6. AlbumId scopes the result to that album's members only.
	album, err := pgdb.Album.Add("scopealbum", "desc", "")
	if err != nil {
		t.Fatalf("could not add album: %v", err)
	}
	if _, err := pgdb.Album.AddPhotos(album.Id, []uuid.UUID{pA.Id, pC.Id}); err != nil {
		t.Fatalf("could not add photos to album: %v", err)
	}
	if photos, err := pgdb.Photo.Select(PhotoFilter{AlbumId: &album.Id}, Range{}, UploadDate); err != nil {
		t.Fatalf("Select by album failed: %v", err)
	} else {
		want := []uuid.UUID{pA.Id, pC.Id} // pA upload 3h, pC upload 1h
		if got := idsOf(photos); !equalIDs(got, want) {
			t.Errorf("album scope: expected %v got %v", want, got)
		}
	}
}

func equalIDs(a, b []uuid.UUID) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// sameIDSet compares two id lists ignoring order — filter results have no
// meaningful order under these tests, only membership.
func sameIDSet(got []uuid.UUID, want ...uuid.UUID) bool {
	if len(got) != len(want) {
		return false
	}
	seen := map[uuid.UUID]bool{}
	for _, id := range got {
		seen[id] = true
	}
	for _, id := range want {
		if !seen[id] {
			return false
		}
	}
	return true
}

func mkEquipPhoto(model, make, lensModel, lensMake string) Photo {
	p := mkSelectPhoto(model, testEquipTime, testEquipTime)
	p.CameraMake = make
	p.LensModel = lensModel
	p.LensMake = lensMake
	return p
}

var testEquipTime = time.Date(2026, 2, 1, 9, 0, 0, 0, time.UTC)

func TestPhotoSelectEquipmentFilters(t *testing.T) {
	pgdb := openAndCreateTestDb(t)
	defer deleteAndCloseTestDb(pgdb, t)

	p1 := mkEquipPhoto("Alpha", "Nikon", "50mm", "Nikon")
	p2 := mkEquipPhoto("Beta", "Nikon", "85mm", "Sigma")
	p3 := mkEquipPhoto("Alpha", "Canon", "50mm", "Canon")
	p4 := mkEquipPhoto("Gamma", "Canon", "24mm", "Canon")

	for _, p := range []Photo{p1, p2, p3, p4} {
		pc := p
		if err := pgdb.Photo.Add(&pc, nil); err != nil {
			t.Fatalf("could not add photo %s: %v", p.Title, err)
		}
	}

	cases := []struct {
		name   string
		filter PhotoFilter
		want   []uuid.UUID
	}{
		{"camera model", PhotoFilter{CameraModel: "Alpha"}, []uuid.UUID{p1.Id, p3.Id}},
		{"camera make", PhotoFilter{CameraMake: "Nikon"}, []uuid.UUID{p1.Id, p2.Id}},
		{"lens model", PhotoFilter{LensModel: "50mm"}, []uuid.UUID{p1.Id, p3.Id}},
		{"lens make", PhotoFilter{LensMake: "Canon"}, []uuid.UUID{p3.Id, p4.Id}},
		{"model AND make", PhotoFilter{CameraModel: "Alpha", CameraMake: "Nikon"}, []uuid.UUID{p1.Id}},
		{"model AND lens make", PhotoFilter{CameraModel: "Alpha", LensMake: "Canon"}, []uuid.UUID{p3.Id}},
		{"no match", PhotoFilter{CameraModel: "Nope"}, nil},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			photos, err := pgdb.Photo.Select(c.filter, Range{}, UploadDate)
			if err != nil {
				t.Fatalf("Select failed: %v", err)
			}
			if !sameIDSet(idsOf(photos), c.want...) {
				t.Errorf("expected %v got %v", c.want, idsOf(photos))
			}
		})
	}

	// A filter combined with an album scope — the guest case: filtering within
	// the photostream. Album holds p1 and p2; only p1 is model Alpha.
	album, err := pgdb.Album.Add("stream", "desc", "")
	if err != nil {
		t.Fatalf("could not add album: %v", err)
	}
	if _, err := pgdb.Album.AddPhotos(album.Id, []uuid.UUID{p1.Id, p2.Id}); err != nil {
		t.Fatalf("could not add photos to album: %v", err)
	}
	if photos, err := pgdb.Photo.Select(PhotoFilter{AlbumId: &album.Id, CameraModel: "Alpha"}, Range{}, UploadDate); err != nil {
		t.Fatalf("Select album+filter failed: %v", err)
	} else if !sameIDSet(idsOf(photos), p1.Id) {
		t.Errorf("album+filter expected [p1] got %v", idsOf(photos))
	}
}
