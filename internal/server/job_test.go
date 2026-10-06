package server

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"google.golang.org/api/drive/v3"
)

// cancelReq builds a PUT /jobs/{jobid}/cancel request with the path value set,
// as the mux would.
func cancelReq(jobId string) *http.Request {
	r := httptest.NewRequest(http.MethodPut, "/api/jobs/"+jobId+"/cancel", nil)
	r.SetPathValue("jobid", jobId)
	return r
}

// TestProcessCancelledWhileQueued: an image job cancelled before the worker picks it
// up ends CANCELLED without touching any file. The job has a nil Drive service, so
// reaching addDrivePhoto would panic.
func TestProcessCancelledWhileQueued(t *testing.T) {
	job := newJob(&mserver{}, "image", 2)
	job.files = []*drive.File{{Id: "a", Name: "a.jpg"}, {Id: "b", Name: "b.jpg"}}
	job.cancel()

	process(job)

	if job.State != StateCancelled {
		t.Errorf("expected state %s, got %s", StateCancelled, job.State)
	}
	if job.NumProcessed != 0 || job.Err != nil {
		t.Errorf("expected nothing processed and no error, got processed=%d err=%v", job.NumProcessed, job.Err)
	}
}

// TestProcessVideoCancelledRemovesStaged: a cancelled video job removes the staged
// local uploads it will never import and records nothing as failed.
func TestProcessVideoCancelledRemovesStaged(t *testing.T) {
	staged := filepath.Join(t.TempDir(), "clip.import")
	if err := os.WriteFile(staged, []byte("video"), 0644); err != nil {
		t.Fatal(err)
	}
	s := &mserver{}
	job := newJob(s, "video", 1)
	job.videoSources = []videoSource{{name: "clip.mp4", stagedPath: staged}}
	job.cancel()

	s.processVideo(job)

	if job.State != StateCancelled {
		t.Errorf("expected state %s, got %s", StateCancelled, job.State)
	}
	if job.NumFailed != 0 || job.NumProcessed != 0 {
		t.Errorf("expected no failed/processed files, got failed=%d processed=%d", job.NumFailed, job.NumProcessed)
	}
	if _, err := os.Stat(staged); !os.IsNotExist(err) {
		t.Errorf("expected staged file to be removed, stat err = %v", err)
	}
}

// TestFinishJobCancelOverridesError: an error caused by the cancel (e.g. an
// interrupted download) reports CANCELLED, not ABORTED.
func TestFinishJobCancelOverridesError(t *testing.T) {
	job := newJob(&mserver{}, "image", 1)
	job.cancel()
	finishJob(job, errors.New("context canceled"))
	if job.State != StateCancelled || job.Err != nil {
		t.Errorf("expected CANCELLED without error, got state=%s err=%v", job.State, job.Err)
	}

	job = newJob(&mserver{}, "image", 1)
	finishJob(job, errors.New("boom"))
	if job.State != StateAborted {
		t.Errorf("expected an uncancelled error to abort, got %s", job.State)
	}
}

func TestHandleCancelJob(t *testing.T) {
	s := &mserver{}

	if _, err := s.handleCancelJob(cancelReq("no-such-job")); err == nil {
		t.Error("expected an error for an unknown job")
	} else if apiErr, ok := err.(*ApiError); !ok || apiErr.Code != http.StatusNotFound {
		t.Errorf("expected a 404 ApiError, got %v", err)
	}

	// A running job gets its context cancelled; the state flips when the worker stops.
	running := newJob(s, "image", 3)
	running.State = StateStarted
	addJob(running)
	res, err := s.handleCancelJob(cancelReq(running.Id))
	if err != nil {
		t.Fatalf("cancel running job: %v", err)
	}
	if snap := res.(*Job); snap.Id != running.Id {
		t.Errorf("expected snapshot of %s, got %s", running.Id, snap.Id)
	}
	if running.ctx.Err() == nil {
		t.Error("expected the running job's context to be cancelled")
	}

	// Cancelling a finished job is a no-op that returns its status.
	done := newJob(s, "image", 1)
	addJob(done)
	finishJob(done, nil)
	res, err = s.handleCancelJob(cancelReq(done.Id))
	if err != nil {
		t.Fatalf("cancel finished job: %v", err)
	}
	if snap := res.(*Job); snap.State != StateFinished {
		t.Errorf("expected finished job to stay %s, got %s", StateFinished, snap.State)
	}
}
