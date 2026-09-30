package dao

import "testing"

// TestImportError covers the import_error store: record (with upsert), lookup by
// md5 (used to skip known-failed files on re-sync), list, and delete-to-retry.
func TestImportError(t *testing.T) {
	pgdb := openAndCreateTestDb(t)
	defer deleteAndCloseTestDb(pgdb, t)

	if pgdb.ImportError.HasMd5("abc") {
		t.Error("no record should exist yet")
	}
	if err := pgdb.ImportError.Record(&ImportError{
		Md5: "abc", DriveId: "d1", Name: "v.mp4", Category: "hdr", Message: "HDR not supported",
	}); err != nil {
		t.Fatalf("record: %v", err)
	}
	if !pgdb.ImportError.HasMd5("abc") {
		t.Error("expected a record for abc")
	}

	// Re-recording the same md5 upserts (newest reason wins).
	if err := pgdb.ImportError.Record(&ImportError{Md5: "abc", Category: "truncated", Name: "v.mp4"}); err != nil {
		t.Fatalf("re-record: %v", err)
	}
	list, err := pgdb.ImportError.List()
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(list) != 1 || list[0].Category != "truncated" {
		t.Errorf("expected 1 record with category 'truncated', got %+v", list)
	}

	// Delete allows a retry.
	if err := pgdb.ImportError.Delete("abc"); err != nil {
		t.Fatalf("delete: %v", err)
	}
	if pgdb.ImportError.HasMd5("abc") {
		t.Error("expected the record to be gone after delete")
	}
}
