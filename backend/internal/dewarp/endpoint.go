package dewarp

import (
	"encoding/json"
	"fmt"
)

// JobInput is what the serverless handler receives. The credentials travel with
// the job because RunPod offers no other per-job secret channel.
type JobInput struct {
	Bucket          string `json:"bucket"`
	EndpointURL     string `json:"endpoint_url,omitempty"`
	Region          string `json:"region"`
	AccessKeyID     string `json:"aws_access_key_id"`
	SecretAccessKey string `json:"aws_secret_access_key"`
	InputPrefix     string `json:"input_prefix"`
	OutputPrefix    string `json:"output_prefix"`
	JPEGQuality     int    `json:"jpeg_quality"`
}

type OutputPage struct {
	InputKey  string `json:"input_key"`
	OutputKey string `json:"output_key"`
	Width     int    `json:"width"`
	Height    int    `json:"height"`
}

type OutputFailure struct {
	InputKey string `json:"input_key"`
	Error    string `json:"error"`
}

type Output struct {
	Pages  []OutputPage    `json:"pages"`
	Failed []OutputFailure `json:"failed"`
}

func decodeOutput(raw json.RawMessage) (Output, error) {
	var output Output
	if err := json.Unmarshal(raw, &output); err != nil {
		return Output{}, fmt.Errorf("the dewarp endpoint returned an unreadable result: %w", err)
	}
	return output, nil
}
