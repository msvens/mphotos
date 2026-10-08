package server

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

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

// TestCancelQueuedJob: cancelling a job the worker hasn't reached reads CANCELLED at
// once and drops its staged upload; the worker later skips it without reviving it.
func TestCancelQueuedJob(t *testing.T) {
	staged := filepath.Join(t.TempDir(), "clip.import")
	if err := os.WriteFile(staged, []byte("video"), 0644); err != nil {
		t.Fatal(err)
	}
	s := &mserver{}
	job := newJob(s, "video", 1)
	job.videoSources = []videoSource{{name: "clip.mp4", stagedPath: staged}}
	addJob(job)

	res, err := s.handleCancelJob(cancelReq(job.Id))
	if err != nil {
		t.Fatalf("cancel queued job: %v", err)
	}
	if snap := res.(*Job); snap.State != StateCancelled {
		t.Errorf("expected queued job to read %s at once, got %s", StateCancelled, snap.State)
	}
	if _, err := os.Stat(staged); !os.IsNotExist(err) {
		t.Errorf("expected staged file to be removed on cancel, stat err = %v", err)
	}

	// The worker dequeues it later: it must stay CANCELLED with nothing processed.
	s.processVideo(job)
	if snap, _ := getJob(job.Id); snap.State != StateCancelled || snap.NumProcessed != 0 {
		t.Errorf("expected worker to skip the cancelled job, got state=%s processed=%d", snap.State, snap.NumProcessed)
	}
}

// TestJobStartKeepsCancelled: a worker starting a job that was cancelled while
// queued doesn't flip it back to STARTED.
func TestJobStartKeepsCancelled(t *testing.T) {
	job := newJob(&mserver{}, "image", 1)
	job.State = StateCancelled
	jobStart(job)
	if job.State != StateCancelled {
		t.Errorf("expected %s to stick, got %s", StateCancelled, job.State)
	}
	job = newJob(&mserver{}, "image", 1)
	jobStart(job)
	if job.State != StateStarted {
		t.Errorf("expected a scheduled job to start, got %s", job.State)
	}
}

// TestFinishedJobIsReaped: an ended job stays queryable for jobRetention, then is
// removed from jobMap.
func TestFinishedJobIsReaped(t *testing.T) {
	old := jobRetention
	jobRetention = 50 * time.Millisecond
	defer func() { jobRetention = old }()

	job := newJob(&mserver{}, "image", 1)
	addJob(job)
	finishJob(job, nil)
	if _, found := getJob(job.Id); !found {
		t.Fatal("expected the job to still be queryable right after it ended")
	}
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if _, found := getJob(job.Id); !found {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Error("expected the ended job to be removed after jobRetention")
}
