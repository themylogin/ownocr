package books

import "testing"

func TestEventCoverURLFollowsTheImageBookKey(t *testing.T) {
	book := Book{ID: 1, StorageKey: "copykey", Stage: StageOCR}

	own := Page{StorageKey: "pagekey"}
	if got := Event(book, &own)["coverUrl"]; got != "/files/copykey/pagekey.480.jpg" {
		t.Errorf("coverUrl = %v, want the key of its own book", got)
	}

	borrowed := "sourcekey"
	shared := Page{StorageKey: "pagekey", ImageBookKey: &borrowed}
	if got := Event(book, &shared)["coverUrl"]; got != "/files/sourcekey/pagekey.480.jpg" {
		t.Errorf("coverUrl = %v, want the key the page names", got)
	}

	empty := ""
	blank := Page{StorageKey: "pagekey", ImageBookKey: &empty}
	if got := Event(book, &blank)["coverUrl"]; got != "/files/copykey/pagekey.480.jpg" {
		t.Errorf("coverUrl = %v, want the key of its own book for an empty value", got)
	}

	if got := Event(book, nil)["coverUrl"]; got != nil {
		t.Errorf("coverUrl = %v, want null with no cover", got)
	}
}
