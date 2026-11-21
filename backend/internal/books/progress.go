package books

// The phases a worker reports. A client shows the text as it is, so the wording
// belongs to the server alone.
const (
	PhaseQueued      = "Waiting in local queue"
	PhaseConverting  = "Converting"
	PhaseUploading   = "Uploading"
	PhaseWaitingAI   = "Waiting for AI"
	PhaseProcessing  = "Processing with AI"
	PhaseDownloading = "Downloading"
)
