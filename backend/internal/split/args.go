package split

// QueueName keeps split off the default queue. One book rewrites every page, so
// the queue runs a single job at a time.
const QueueName = "split"

// MaxAttempts is the River retry count for one split job.
const MaxAttempts = 3

// Args names the copy the worker fills, never the source book.
type Args struct {
	BookID int64 `json:"bookId"`
}

func (Args) Kind() string { return "split" }
