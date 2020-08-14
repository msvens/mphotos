package server

import (
	"github.com/msvens/mexif"
	"google.golang.org/api/drive/v3"
	"sync"
)

const StateScheduled = "SCHEDULED"
const StateStarted = "STARTED"
const StateFinished = "FINISHED"
const StateAborted = "ABORTED"

type Job struct {
	Id           string `json:"id"`
	State        string `json:"state"`
	Percent      int    `json:"percent"`
	files        []*drive.File
	s            *mserver
	NumFiles     int       `json:"numFiles"`
	NumProcessed int       `json:"numProcessed"`
	Err          *ApiError `json:"error,omitempty"`
}

var jobChan = make(chan *Job, 10)
var wg sync.WaitGroup
var jobMap = make(map[string]*Job)

func worker(jobChan <-chan *Job) {

	defer wg.Done()

	for job := range jobChan {
		job.s.l.Infow("Processing job", "jobid", job.Id, "files", job.NumFiles)
		process(job)
	}
}

func process(job *Job) {

	tool, err := mexif.NewMExifTool()
	defer tool.Close()

	if err != nil {
		finishJob(job, err)
		return
	}

	job.State = StateStarted

	var percent = 100 / job.NumFiles

	for _, f := range job.files {
		if _, err := addPhoto(job.s, f, tool); err != nil {
			finishJob(job, err)
			return
		}
		job.Percent = job.Percent + percent
		job.NumProcessed = job.NumProcessed + 1
		job.s.l.Debugw("", "jobid", job.Id, "progress", job.Percent)
	}
	finishJob(job, nil)
}

func finishJob(job *Job, err error) {
	job.files = nil
	job.s = nil
	if err != nil {
		job.State = StateAborted
		job.Err = ResolveError(err)
	} else {
		job.Percent = 100
		job.State = StateFinished
	}
}
