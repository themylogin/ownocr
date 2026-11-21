package books

import "testing"

func titles(actions []Action) []string {
	names := make([]string, 0, len(actions))
	for _, action := range actions {
		names = append(names, action.Title)
	}
	return names
}

func equal(got, want []string) bool {
	if len(got) != len(want) {
		return false
	}
	for index := range got {
		if got[index] != want[index] {
			return false
		}
	}
	return true
}

func TestActionsFor(t *testing.T) {
	cases := []struct {
		stage string
		want  []string
	}{
		{StageUploaded, []string{"Dewarp", "Split", "OCR"}},
		{StageDewarped, []string{"Split", "OCR"}},
		{StageSplit, []string{"OCR"}},
		{StageOCR, []string{}},
		{"nonsense", []string{}},
	}

	for _, test := range cases {
		got := titles(ActionsFor(test.stage))
		if !equal(got, test.want) {
			t.Errorf("ActionsFor(%q) = %v, want %v", test.stage, got, test.want)
		}
	}
}

func TestOffersAction(t *testing.T) {
	cases := []struct {
		stage  string
		action string
		want   bool
	}{
		{StageUploaded, ActionDewarp, true},
		{StageUploaded, ActionSplit, true},
		{StageUploaded, ActionOCR, true},
		{StageDewarped, ActionDewarp, false},
		{StageDewarped, ActionSplit, true},
		{StageSplit, ActionSplit, false},
		{StageSplit, ActionOCR, true},
		{StageOCR, ActionSplit, false},
		{StageOCR, ActionOCR, false},
		{StageUploaded, "nonsense", false},
	}

	for _, test := range cases {
		if got := OffersAction(test.stage, test.action); got != test.want {
			t.Errorf("OffersAction(%q, %q) = %v, want %v",
				test.stage, test.action, got, test.want)
		}
	}
}

func TestActionByID(t *testing.T) {
	action, ok := ActionByID(ActionDewarp)
	if !ok || action.Stage != StageDewarped {
		t.Errorf("ActionByID(dewarp) = %+v, %v", action, ok)
	}

	if _, ok := ActionByID("nonsense"); ok {
		t.Error("ActionByID(nonsense) reported an action")
	}
}

func TestFirstStage(t *testing.T) {
	if FirstStage() != StageUploaded {
		t.Errorf("FirstStage() = %q, want %q", FirstStage(), StageUploaded)
	}
}

func TestStageOrder(t *testing.T) {
	want := []string{StageUploaded, StageDewarped, StageSplit, StageOCR}
	got := make([]string, 0, len(Stages))
	for _, stage := range Stages {
		got = append(got, stage.ID)
	}
	if !equal(got, want) {
		t.Errorf("Stages = %v, want %v", got, want)
	}

	if StageTitle(StageSplit) != "Split" {
		t.Errorf("StageTitle(split) = %q, want %q", StageTitle(StageSplit), "Split")
	}
}

// The Split action lands on its own stage. Landing it on `done` again would put
// split books back in the tab that names the end of the pipeline.
func TestSplitLandsOnTheSplitStage(t *testing.T) {
	action, ok := ActionByID(ActionSplit)
	if !ok {
		t.Fatal("the split action is missing")
	}
	if action.Stage != StageSplit {
		t.Errorf("the split action lands on %q, want %q", action.Stage, StageSplit)
	}
}

// The OCR action ends the pipeline, so its copy lands on `done`. It offers no
// rollback: it reads a page without changing it, so the page a rollback would
// install is the page the copy already shows.
func TestOCRLandsOnTheOCRStage(t *testing.T) {
	action, ok := ActionByID(ActionOCR)
	if !ok {
		t.Fatal("the ocr action is missing")
	}
	if action.Stage != StageOCR {
		t.Errorf("the ocr action lands on %q, want %q", action.Stage, StageOCR)
	}
	if action.OffersRollback {
		t.Error("the ocr action offers a rollback")
	}
}

// Every action must land on a stage that exists, or ActionsFor silently drops
// it from every menu.
func TestEveryActionHasAStage(t *testing.T) {
	for _, action := range Actions {
		if !HasStage(action.Stage) {
			t.Errorf("action %q lands on unknown stage %q", action.ID, action.Stage)
		}
	}
}
