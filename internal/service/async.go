package service

import (
	"github.com/google/uuid"
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
	ps           *PhotoService
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
		logger.Infow("Processing job", "jobid", job.Id, "files", job.NumFiles)
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
		if _, err := job.ps.AddPhoto(f, tool); err != nil {
			finishJob(job, err)
			return
		}
		job.Percent = job.Percent + percent
		job.NumProcessed = job.NumProcessed + 1
		logger.Debugw("", "jobid", job.Id, "progress", job.Percent)
	}
	finishJob(job, nil)
}

func finishJob(job *Job, err error) {
	job.files = nil
	job.ps = nil
	if err != nil {
		job.State = StateAborted
		job.Err = ResolveError(err)
	} else {
		job.Percent = 100
		job.State = StateFinished
	}

}

func (ps *PhotoService) JobStatus(id string) (*Job, error) {
	if job, found := jobMap[id]; found {
		return job, nil
	} else {
		return nil, NotFoundError("job not found")
	}
}

func (ps *PhotoService) ScheduleAddPhotos() (*Job, error) {
	fl, err := ps.ListDriveFiles()
	if err != nil {
		return nil, err
	}
	job := Job{}
	job.Id = uuid.New().String()
	job.files = fl
	job.ps = ps
	job.NumFiles = len(fl)
	job.State = StateScheduled
	jobMap[job.Id] = &job
	jobChan <- &job
	return &job, nil
}
