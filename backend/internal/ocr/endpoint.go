package ocr

import (
	"encoding/json"
	"fmt"
	"path"
	"strings"
)

// The engines that read a page. ModeV6 is PP-OCRv6: fast, with a box and a
// confidence for every line, and no polytonic Greek. ModeVL is PaddleOCR-VL: it
// reads difficult pages and reports no recognition confidence. Both run on the
// endpoint.
//
// ModeGLM is GLM-5.3 Flash, which the server reads itself against a metered
// API. It reaches no endpoint job, so it is on no profile the endpoint sees.
const (
	ModeV6  = "v6"
	ModeVL  = "vl"
	ModeGLM = "glm"
)

// The stored results no engine produced. Each holds the line boxes of PP-OCRv6
// filled with the text of one reading engine, derived from the two results of
// that page. A page read by both reading engines holds both, so a reader can
// answer either. Neither ever reaches the endpoint, because no profile lists it.
const (
	ModelMerged    = "merged"
	ModelGLMMerged = "glm_merged"
)

// The profiles a user picks from. A profile names the engines it runs, so Extra
// keeps the line boxes of v6 beside the text of VL.
//
// ProfileUltra belongs to the rescan alone. A whole book is read by the engines
// the endpoint runs; Ultra reads a page the endpoint already failed, one page at
// a time and at a price per page.
const (
	ProfileNormal = "v6"
	ProfileExtra  = "vl"
	ProfileUltra  = "glm"
)

var profileModes = map[string][]string{
	ProfileNormal: {ModeV6},
	ProfileExtra:  {ModeV6, ModeVL},
}

// rescanModes names the engines a rescan of a profile runs. Extra runs the
// visual engine alone: the PP-OCRv6 result of the page is already stored, and
// PP-OCRv6 reads a page the same way every time, so a second reading returns
// what the store holds. Ultra runs the reading model alone for the same reason.
var rescanModes = map[string][]string{
	ProfileNormal: {ModeV6},
	ProfileExtra:  {ModeVL},
	ProfileUltra:  {ModeGLM},
}

// ModesOf answers the engines the OCR action of a profile runs. The second
// answer is false for a profile the system does not know.
func ModesOf(profile string) ([]string, bool) {
	return modesIn(profileModes, profile)
}

// RescanModesOf answers the engines a rescan of a profile runs. A profile means
// one set of engines to the OCR action and another to a rescan.
func RescanModesOf(profile string) ([]string, bool) {
	return modesIn(rescanModes, profile)
}

func modesIn(known map[string][]string, profile string) ([]string, bool) {
	modes, ok := known[profile]
	if !ok {
		return nil, false
	}
	return append([]string(nil), modes...), true
}

// KnownProfile answers whether the OCR action of a whole book reads with this
// profile. It refuses the rescan-only profile, which RescanOnlyProfile tells
// from a name the system does not know at all.
func KnownProfile(profile string) bool {
	_, ok := profileModes[profile]
	return ok
}

// KnownRescanProfile answers whether a rescan reads with this profile. A rescan
// accepts every profile the system defines, the rescan-only one included.
func KnownRescanProfile(profile string) bool {
	_, ok := rescanModes[profile]
	return ok
}

// RescanOnlyProfile answers whether a rescan reads with this profile and the OCR
// action does not, so a refusal can say which of the two it is.
func RescanOnlyProfile(profile string) bool {
	return KnownRescanProfile(profile) && !KnownProfile(profile)
}

// Profiles lists the profiles the OCR action offers, Normal first. The interface
// offers them in this order.
func Profiles() []string { return []string{ProfileNormal, ProfileExtra} }

// RescanProfiles lists the profiles a rescan offers, best last. The rescan
// dialog offers them in this order.
func RescanProfiles() []string {
	return []string{ProfileNormal, ProfileExtra, ProfileUltra}
}

// JobInput is what the serverless handler receives. The credentials travel with
// the job because RunPod offers no other per-job secret channel. Modes names the
// engines to run; the handler refuses a job that names none.
// Temperature and TopP name the sampling the visual engine runs with. Both are
// absent unless a caller sets them, which leaves the endpoint on the sampling it
// picks for itself.
type JobInput struct {
	Bucket          string   `json:"bucket"`
	EndpointURL     string   `json:"endpoint_url,omitempty"`
	Region          string   `json:"region"`
	AccessKeyID     string   `json:"aws_access_key_id"`
	SecretAccessKey string   `json:"aws_secret_access_key"`
	InputPrefix     string   `json:"input_prefix"`
	OutputPrefix    string   `json:"output_prefix"`
	Modes           []string `json:"modes"`
	Temperature     *float64 `json:"temperature,omitempty"`
	TopP            *float64 `json:"top_p,omitempty"`
}

// OutputPage is one engine reading one page. The endpoint answers one of these
// per engine per page, so two entries can share an input key.
//
// Nothing in the entry names its engine. The names of the objects it wrote do:
// v6 writes `<name>.v6.json` and `<name>.v6.txt`, VL writes `<name>.md` and
// `<name>.json`.
type OutputPage struct {
	InputKey   string   `json:"input_key"`
	OutputKeys []string `json:"output_keys"`
	Chars      int      `json:"chars"`
	Blocks     int      `json:"blocks"`
	Lines      int      `json:"lines"`
	Seconds    float64  `json:"seconds"`
}

// Mode answers the engine that produced this entry.
func (p OutputPage) Mode() string {
	for _, key := range p.OutputKeys {
		if strings.HasSuffix(path.Base(key), ".v6.txt") ||
			strings.HasSuffix(path.Base(key), ".v6.json") {
			return ModeV6
		}
	}
	if len(p.OutputKeys) > 0 {
		return ModeVL
	}
	return ""
}

// TextKey answers the object holding the text of this engine, and JSONKey the
// object holding its full result. The second answer is false when the entry
// does not carry it.
func (p OutputPage) TextKey() (string, bool) {
	if p.Mode() == ModeV6 {
		return p.keyWithSuffix(".v6.txt")
	}
	return p.keyWithSuffix(".md")
}

func (p OutputPage) JSONKey() (string, bool) {
	if p.Mode() == ModeV6 {
		return p.keyWithSuffix(".v6.json")
	}
	// `.v6.json` also ends in `.json`, but a VL entry never carries one.
	return p.keyWithSuffix(".json")
}

func (p OutputPage) keyWithSuffix(suffix string) (string, bool) {
	for _, key := range p.OutputKeys {
		if strings.HasSuffix(path.Base(key), suffix) {
			return key, true
		}
	}
	return "", false
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
		return Output{}, fmt.Errorf("the OCR endpoint returned an unreadable result: %w", err)
	}
	return output, nil
}
