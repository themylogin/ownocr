package ocr

import "testing"

func TestRescanIsItsOwnJobKind(t *testing.T) {
	rescan := (RescanArgs{}).Kind()

	if rescan == (Args{}).Kind() {
		t.Fatalf("the rescan job and the OCR job share the kind %q", rescan)
	}
	if rescan != "ocr_rescan" {
		t.Errorf("the rescan job kind is %q, want %q", rescan, "ocr_rescan")
	}
}
