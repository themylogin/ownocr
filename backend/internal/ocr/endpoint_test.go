package ocr

import (
	"strings"
	"testing"

	"github.com/themylogin/ownocr/backend/internal/dewarp"
)

// A recorded answer of runpod/paddleocr, taken from a real two engine job. The
// worker pairs a result by its input key and its engine, so the shape of this
// answer is a contract: two entries share one input key, and only the names of
// the objects they wrote say which engine produced them.
const recordedAnswer = `{
  "pages": [
    {"input_key": "ocr/abc/in/0001.jpg",
     "output_keys": ["ocr/abc/out/0001.v6.json", "ocr/abc/out/0001.v6.txt"],
     "chars": 4105, "lines": 71, "seconds": 3.13},
    {"input_key": "ocr/abc/in/0001.jpg",
     "output_keys": ["ocr/abc/out/0001.md", "ocr/abc/out/0001.json"],
     "chars": 4053, "blocks": 11, "seconds": 12.75}
  ],
  "failed": [
    {"input_key": "ocr/abc/in/0003.jpg", "error": "cannot identify image file"}
  ]
}`

func TestDecodeOutputReadsTheHandlerAnswer(t *testing.T) {
	output, err := decodeOutput([]byte(recordedAnswer))
	if err != nil {
		t.Fatalf("decodeOutput: %v", err)
	}

	if len(output.Pages) != 2 {
		t.Fatalf("got %d results, want 2", len(output.Pages))
	}

	v6, vl := output.Pages[0], output.Pages[1]
	if v6.InputKey != vl.InputKey {
		t.Fatalf("the two results describe different pages: %q and %q", v6.InputKey, vl.InputKey)
	}

	if v6.Mode() != ModeV6 {
		t.Errorf("the first result reports the engine %q, want %q", v6.Mode(), ModeV6)
	}
	if vl.Mode() != ModeVL {
		t.Errorf("the second result reports the engine %q, want %q", vl.Mode(), ModeVL)
	}

	if key, ok := v6.TextKey(); !ok || key != "ocr/abc/out/0001.v6.txt" {
		t.Errorf("the v6 text is %q (%v)", key, ok)
	}
	if key, ok := v6.JSONKey(); !ok || key != "ocr/abc/out/0001.v6.json" {
		t.Errorf("the v6 result is %q (%v)", key, ok)
	}
	if key, ok := vl.TextKey(); !ok || key != "ocr/abc/out/0001.md" {
		t.Errorf("the vl text is %q (%v)", key, ok)
	}
	// `.v6.json` also ends in `.json`, so a VL entry must not claim one.
	if key, ok := vl.JSONKey(); !ok || key != "ocr/abc/out/0001.json" {
		t.Errorf("the vl result is %q (%v)", key, ok)
	}

	if v6.Chars != 4105 || v6.Lines != 71 || v6.Seconds != 3.13 {
		t.Errorf("the v6 result lost its work report: %+v", v6)
	}
	if vl.Chars != 4053 || vl.Blocks != 11 {
		t.Errorf("the vl result lost its work report: %+v", vl)
	}

	if len(output.Failed) != 1 {
		t.Fatalf("got %d failures, want 1", len(output.Failed))
	}
	if output.Failed[0].InputKey != "ocr/abc/in/0003.jpg" {
		t.Errorf("failure = %+v", output.Failed[0])
	}
}

func TestModesOfEveryProfile(t *testing.T) {
	normal, ok := ModesOf(ProfileNormal)
	if !ok || len(normal) != 1 || normal[0] != ModeV6 {
		t.Errorf("the normal profile runs %v, want the v6 engine alone", normal)
	}

	// Extra keeps the line boxes and the confidence of v6, which VL does not
	// report, beside the text of VL.
	extra, ok := ModesOf(ProfileExtra)
	if !ok || len(extra) != 2 || extra[0] != ModeV6 || extra[1] != ModeVL {
		t.Errorf("the extra profile runs %v, want both engines", extra)
	}

	if _, ok := ModesOf("nonsense"); ok {
		t.Error("an unknown profile named engines to run")
	}
	if _, ok := ModesOf(""); ok {
		t.Error("an empty profile named engines to run")
	}

	if !KnownProfile(ProfileNormal) || !KnownProfile(ProfileExtra) {
		t.Error("a known profile was refused")
	}
	if KnownProfile("nonsense") {
		t.Error("an unknown profile was accepted")
	}
}

// A caller must not be able to change the engines of a profile through the
// slice it is given.
func TestModesOfAnswersACopy(t *testing.T) {
	first, _ := ModesOf(ProfileExtra)
	first[0] = "tampered"

	second, _ := ModesOf(ProfileExtra)
	if second[0] != ModeV6 {
		t.Errorf("the extra profile now runs %v", second)
	}
}

// A rescan of the extra profile runs the visual engine alone. The v6 result of
// the page is already stored, so the endpoint is never asked for it again.
func TestRescanModesOfEveryProfile(t *testing.T) {
	normal, ok := RescanModesOf(ProfileNormal)
	if !ok || len(normal) != 1 || normal[0] != ModeV6 {
		t.Errorf("a normal rescan runs %v, want the v6 engine alone", normal)
	}

	extra, ok := RescanModesOf(ProfileExtra)
	if !ok || len(extra) != 1 || extra[0] != ModeVL {
		t.Errorf("an extra rescan runs %v, want the vl engine alone", extra)
	}

	ultra, ok := RescanModesOf(ProfileUltra)
	if !ok || len(ultra) != 1 || ultra[0] != ModeGLM {
		t.Errorf("an Ultra rescan runs %v, want the glm engine alone", ultra)
	}

	if _, ok := RescanModesOf("nonsense"); ok {
		t.Error("an unknown profile named engines to run")
	}
	if _, ok := RescanModesOf(""); ok {
		t.Error("an empty profile named engines to run")
	}
}

func TestRescanModesOfAnswersACopy(t *testing.T) {
	first, _ := RescanModesOf(ProfileExtra)
	first[0] = "tampered"

	second, _ := RescanModesOf(ProfileExtra)
	if second[0] != ModeVL {
		t.Errorf("an extra rescan now runs %v", second)
	}
}

func TestDecodeOutputNamesTheEndpoint(t *testing.T) {
	_, err := decodeOutput([]byte(`{"pages": "not a list"}`))
	if err == nil {
		t.Fatal("an unreadable result was accepted")
	}
	if !strings.Contains(err.Error(), "the OCR endpoint") {
		t.Errorf("error %q does not name the endpoint", err)
	}
}

// Two actions run on one book, so their objects must never meet.
func TestKeysDifferFromDewarp(t *testing.T) {
	const bookKey = "abc"

	if InputKey(bookKey, 1, "jpg") != "ocr/abc/in/0001.jpg" {
		t.Errorf("InputKey = %q", InputKey(bookKey, 1, "jpg"))
	}
	if OutputPrefix(bookKey) != "ocr/abc/out/" {
		t.Errorf("OutputPrefix = %q", OutputPrefix(bookKey))
	}
	if strings.HasPrefix(Prefix(bookKey), dewarp.Prefix(bookKey)) ||
		strings.HasPrefix(dewarp.Prefix(bookKey), Prefix(bookKey)) {
		t.Errorf("the OCR prefix %q and the dewarp prefix %q overlap",
			Prefix(bookKey), dewarp.Prefix(bookKey))
	}
}

// The Ultra profile belongs to the rescan alone. A whole book is read by the
// engines the endpoint runs, and Ultra reads neither on the endpoint nor for
// free.
func TestTheUltraProfileIsForTheRescanAlone(t *testing.T) {
	if _, ok := ModesOf(ProfileUltra); ok {
		t.Error("the OCR action of a whole book runs the Ultra profile")
	}
	if KnownProfile(ProfileUltra) {
		t.Error("the OCR action accepts the Ultra profile")
	}

	if !KnownRescanProfile(ProfileUltra) {
		t.Error("a rescan refuses the Ultra profile")
	}
	if !KnownRescanProfile(ProfileNormal) || !KnownRescanProfile(ProfileExtra) {
		t.Error("a rescan refuses a profile the OCR action runs")
	}
	if KnownRescanProfile("nonsense") || KnownRescanProfile("") {
		t.Error("a rescan accepts a profile the system does not define")
	}

	// A refusal says which of the two it is, so the OCR action can name the
	// reason rather than call a defined profile unknown.
	if !RescanOnlyProfile(ProfileUltra) {
		t.Error("the Ultra profile does not read as rescan-only")
	}
	for _, profile := range []string{ProfileNormal, ProfileExtra, "nonsense", ""} {
		if RescanOnlyProfile(profile) {
			t.Errorf("%q reads as rescan-only", profile)
		}
	}
}

func TestRescanProfilesOffersEveryProfileUltraLast(t *testing.T) {
	want := []string{ProfileNormal, ProfileExtra, ProfileUltra}
	got := RescanProfiles()
	if len(got) != len(want) {
		t.Fatalf("a rescan offers %v, want %v", got, want)
	}
	for i, profile := range want {
		if got[i] != profile {
			t.Fatalf("a rescan offers %v, want %v", got, want)
		}
	}

	// The OCR dialog offers no Ultra.
	for _, profile := range Profiles() {
		if profile == ProfileUltra {
			t.Error("the OCR action offers the Ultra profile")
		}
	}
}
