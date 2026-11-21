package dewarp

import "fmt"

// QueueName keeps dewarp off the default queue. One book pushes gigabytes to
// object storage, so the queue runs a single job at a time.
const QueueName = "dewarp"

// MaxAttempts is the River retry count for one dewarp job.
const MaxAttempts = 3

// Args names the copy the worker fills, never the source book.
type Args struct {
	BookID int64 `json:"bookId"`
}

func (Args) Kind() string { return "dewarp" }

func Prefix(bookKey string) string       { return "dewarp/" + bookKey + "/" }
func InputPrefix(bookKey string) string  { return Prefix(bookKey) + "in/" }
func OutputPrefix(bookKey string) string { return Prefix(bookKey) + "out/" }

// InputKey pads the page index, so a lexical sort of the bucket listing is the
// page order. The endpoint's own ordering is never trusted.
func InputKey(bookKey string, pageIndex int, ext string) string {
	return fmt.Sprintf("%s%04d.%s", InputPrefix(bookKey), pageIndex, ext)
}
