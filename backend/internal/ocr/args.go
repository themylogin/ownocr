package ocr

import "fmt"

// QueueName keeps OCR off the default queue. One book pushes gigabytes to
// object storage, so the queue runs a single job at a time.
const QueueName = "ocr"

// MaxAttempts is the River retry count for one OCR job.
const MaxAttempts = 3

// Args names the copy the worker fills, never the source book.
type Args struct {
	BookID int64 `json:"bookId"`
}

func (Args) Kind() string { return "ocr" }

// Root is the first segment of every object an OCR job reads or writes. It is
// what tells an OCR job from a dewarp job in the bucket.
const Root = "ocr/"

func Prefix(bookKey string) string       { return Root + bookKey + "/" }
func InputPrefix(bookKey string) string  { return Prefix(bookKey) + "in/" }
func OutputPrefix(bookKey string) string { return Prefix(bookKey) + "out/" }

// InputKey pads the page index, so a lexical sort of the bucket listing is the
// page order. The endpoint's own ordering is never trusted.
func InputKey(bookKey string, pageIndex int, ext string) string {
	return fmt.Sprintf("%s%04d.%s", InputPrefix(bookKey), pageIndex, ext)
}

// RescanArgs names a book already read, the pages of it to read again, and the
// profile to read them with. The profile describes this rescan alone and is
// never the profile the book records. It runs on the OCR queue, so a rescan and
// a whole book reading never reach the endpoint at once.
type RescanArgs struct {
	BookID      int64  `json:"bookId"`
	PageIndexes []int  `json:"pageIndexes"`
	Profile     string `json:"profile"`
}

func (RescanArgs) Kind() string { return "ocr_rescan" }
