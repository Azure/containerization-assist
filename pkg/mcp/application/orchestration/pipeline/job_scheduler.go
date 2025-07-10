package pipeline

import "context"

// JobScheduler manages job submission and lifecycle
type JobScheduler interface {
	// Submit adds a new job to the queue
	Submit(job *Job) error

	// Get retrieves a job by ID
	Get(jobID string) (*Job, error)

	// List returns jobs matching the filter criteria
	List(status JobStatus) []*Job

	// Cancel attempts to cancel a pending or running job
	Cancel(jobID string) error
}

// jobScheduler implements JobScheduler
type jobScheduler struct {
	service Service
}

// NewJobScheduler creates a new JobScheduler service
func NewJobScheduler(service Service) JobScheduler {
	return &jobScheduler{
		service: service,
	}
}

func (j *jobScheduler) Submit(job *Job) error {
	return j.service.SubmitJob(context.Background(), job)
}

func (j *jobScheduler) Get(jobID string) (*Job, error) {
	job, found := j.service.GetJob(context.Background(), jobID)
	if !found {
		return nil, ErrJobNotFound
	}
	return job, nil
}

func (j *jobScheduler) List(status JobStatus) []*Job {
	return j.service.ListJobs(context.Background(), status)
}

func (j *jobScheduler) Cancel(jobID string) error {
	return j.service.CancelJob(context.Background(), jobID)
}
