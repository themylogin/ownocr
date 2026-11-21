package books

type Stage struct {
	ID    string `json:"id"`
	Title string `json:"title"`
}

// A stage names what a book already is, not the work that comes next. Order
// defines the tab order in the interface.
//
// `ocr` names the end of the pipeline. A book that carries it has been read.
var Stages = []Stage{
	{ID: "uploaded", Title: "Uploaded"},
	{ID: "dewarped", Title: "Dewarped"},
	{ID: "split", Title: "Split"},
	{ID: "ocr", Title: "OCR"},
}

const (
	StageUploaded = "uploaded"
	StageDewarped = "dewarped"
	StageSplit    = "split"
	StageOCR      = "ocr"
)

// An action names the work that moves a book on. Stage is the stage its copy
// carries, which is what places the action after a stage in the tab order.
//
// OffersRollback declares that a single page of the copy can return to the page
// it came from. It describes the contract of the action, not the state of its
// worker. Only Dewarp declares it: Split makes two pages from one spread, and
// nothing puts them back; OCR reads a page without changing it, so the page it
// would install is the page it already shows.
type Action struct {
	ID             string `json:"id"`
	Title          string `json:"title"`
	Stage          string `json:"-"`
	OffersRollback bool   `json:"-"`
}

var Actions = []Action{
	{ID: "dewarp", Title: "Dewarp", Stage: StageDewarped, OffersRollback: true},
	{ID: "split", Title: "Split", Stage: StageSplit},
	{ID: "ocr", Title: "OCR", Stage: StageOCR},
}

const (
	ActionDewarp = "dewarp"
	ActionSplit  = "split"
	ActionOCR    = "ocr"
)

func FirstStage() string {
	return Stages[0].ID
}

func HasStage(id string) bool {
	return stageIndex(id) >= 0
}

func StageTitle(id string) string {
	for _, stage := range Stages {
		if stage.ID == id {
			return stage.Title
		}
	}
	return id
}

func ActionByID(id string) (Action, bool) {
	for _, action := range Actions {
		if action.ID == id {
			return action, true
		}
	}
	return Action{}, false
}

// ActionsFor returns every action whose copy lands after the given stage. An
// unknown stage offers nothing.
func ActionsFor(stage string) []Action {
	from := stageIndex(stage)
	if from < 0 {
		return nil
	}

	offered := make([]Action, 0, len(Actions))
	for _, action := range Actions {
		if stageIndex(action.Stage) > from {
			offered = append(offered, action)
		}
	}
	return offered
}

func OffersAction(stage, actionID string) bool {
	for _, action := range ActionsFor(stage) {
		if action.ID == actionID {
			return true
		}
	}
	return false
}

func stageIndex(id string) int {
	for index, stage := range Stages {
		if stage.ID == id {
			return index
		}
	}
	return -1
}
