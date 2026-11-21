package ingest

const QueueName = "ingest"

type Args struct {
	BookID int64 `json:"bookId"`
}

func (Args) Kind() string { return "ingest" }
