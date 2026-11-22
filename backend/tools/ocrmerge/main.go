// Command ocrmerge writes the merged result of every page that holds the two
// readings a merge needs, from the rows the database already holds. It takes one
// book under -book, or every book under -all.
//
// A page read before the merge existed holds no merged row, and a page read
// before a fix to the merge holds a row that fix would change. Retrying either
// would upload the page to the endpoint again and spend GPU time to recompute
// results the database already has — or, for the reading read on a metered API,
// spend money to read a page whose text is already stored. So this walks the
// stored rows instead.
//
// It chooses by what a page holds and not by the profile its book records. A
// rescan gives one page of a `v6` copy the readings of another engine, so the
// profile of a book says nothing about which of its pages can be merged.
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/themylogin/ownocr/backend/internal/books"
	"github.com/themylogin/ownocr/backend/internal/ocr"
	"github.com/themylogin/ownocr/backend/internal/ocrmerge"
)

func main() {
	database := flag.String("database", os.Getenv("DATABASE_URL"), "database URL")
	bookID := flag.Int64("book", 0, "id of the book to merge the pages of")
	all := flag.Bool("all", false, "merge every page of every book that holds the readings for it")
	dryRun := flag.Bool("dry-run", false, "report what would be written, write nothing")
	flag.Parse()

	if err := run(context.Background(), *database, *bookID, *all, *dryRun); err != nil {
		fmt.Fprintln(os.Stderr, "ocrmerge:", err)
		os.Exit(1)
	}
}

// merges names the work: for each merged reading, the engine whose text it
// carries. The box engine is the same for both.
var merges = []struct{ Model, TextEngine string }{
	{ocr.ModelMerged, ocr.ModeVL},
	{ocr.ModelGLMMerged, ocr.ModeGLM},
}

func textEngines() []string {
	engines := make([]string, 0, len(merges))
	for _, merge := range merges {
		engines = append(engines, merge.TextEngine)
	}
	return engines
}

// tally is what one run did, per merged reading.
type tally struct {
	pages   int
	skipped int
	counts  ocrmerge.Counts
}

func (t *tally) add(counts ocrmerge.Counts) {
	t.pages++
	t.counts.Lines += counts.Lines
	t.counts.FromText += counts.FromText
	t.counts.FromBoxes += counts.FromBoxes
	t.counts.Estimated += counts.Estimated
	t.counts.Discarded += counts.Discarded
	t.counts.DroppedChars += counts.DroppedChars
	t.counts.Levelled += counts.Levelled
}

func run(ctx context.Context, database string, bookID int64, all, dryRun bool) error {
	if database == "" {
		return fmt.Errorf("no database URL: pass -database or set DATABASE_URL")
	}
	if bookID == 0 && !all {
		return fmt.Errorf("no work: pass -book or -all")
	}
	if bookID != 0 && all {
		return fmt.Errorf("-book and -all name different work: pass one of them")
	}

	pool, err := pgxpool.New(ctx, database)
	if err != nil {
		return err
	}
	defer pool.Close()

	store := books.NewStore(pool)

	pages, err := pagesToMerge(ctx, pool, bookID)
	if err != nil {
		return err
	}
	if len(pages) == 0 {
		if bookID != 0 {
			return fmt.Errorf("no page of book %d holds a line reading beside the text of "+
				"another engine, so there is nothing to merge", bookID)
		}
		fmt.Println("no page holds the two readings a merge needs, so there is nothing to merge")
		return nil
	}

	byBook := map[int64]map[string]*tally{}
	whole := map[string]*tally{}

	for _, page := range pages {
		if byBook[page.BookID] == nil {
			byBook[page.BookID] = map[string]*tally{}
		}

		for _, merge := range merges {
			counts, merged, err := mergePage(ctx, store, page, merge.Model, merge.TextEngine, dryRun)
			if err != nil {
				// One page that will not merge costs that page and no other. A
				// merge derives a reading from two the database holds, so a page
				// it cannot read holds stored rows that are not what they should
				// be — which is worth reporting and not worth stopping for.
				fmt.Fprintf(os.Stderr, "skipped book %d page %d (%s): %v\n",
					page.BookID, page.PageIndex, merge.Model, err)
				countIn(byBook[page.BookID], merge.Model).skipped++
				countIn(whole, merge.Model).skipped++
				continue
			}
			if !merged {
				continue
			}

			countIn(byBook[page.BookID], merge.Model).add(counts)
			countIn(whole, merge.Model).add(counts)
		}
	}

	for _, bookID := range sortedKeys(byBook) {
		book, err := store.Get(ctx, bookID)
		name := fmt.Sprintf("book %d", bookID)
		if err == nil {
			name = fmt.Sprintf("book %d (%s)", bookID, book.Name)
		}
		report(name, byBook[bookID])
	}

	if len(byBook) > 1 {
		report(fmt.Sprintf("%d books", len(byBook)), whole)
	}
	if dryRun {
		fmt.Println("dry run: nothing was written")
	}
	return nil
}

func countIn(tallies map[string]*tally, model string) *tally {
	if tallies[model] == nil {
		tallies[model] = &tally{}
	}
	return tallies[model]
}

func sortedKeys(byBook map[int64]map[string]*tally) []int64 {
	ids := make([]int64, 0, len(byBook))
	for id := range byBook {
		ids = append(ids, id)
	}
	for i := 1; i < len(ids); i++ {
		for j := i; j > 0 && ids[j] < ids[j-1]; j-- {
			ids[j], ids[j-1] = ids[j-1], ids[j]
		}
	}
	return ids
}

func report(subject string, tallies map[string]*tally) {
	for _, merge := range merges {
		held := tallies[merge.Model]
		if held == nil || (held.pages == 0 && held.skipped == 0) {
			continue
		}

		counts := held.counts
		fmt.Printf("%s, %s: %d pages, %d lines, %d from %s, %d from %s, "+
			"%d placed, %d rows levelled, %d boxes discarded, %d runes dropped\n",
			subject, merge.Model, held.pages, counts.Lines,
			counts.FromText, merge.TextEngine, counts.FromBoxes, ocr.ModeV6,
			counts.Estimated, counts.Levelled, counts.Discarded, counts.DroppedChars)
		if held.skipped > 0 {
			fmt.Printf("%s, %s: %d pages were skipped, and every one of them is above\n",
				subject, merge.Model, held.skipped)
		}
	}
}

// pagesToMerge answers every page that holds a line reading beside the text of
// another engine, in book and page order. It asks what a page holds, so a page a
// rescan gave a second reading to is found whatever profile its book records.
func pagesToMerge(ctx context.Context, pool *pgxpool.Pool, bookID int64) ([]books.Page, error) {
	where := ""
	args := []any{ocr.ModeV6, textEngines()}
	if bookID != 0 {
		where = " AND page.book_id = $3"
		args = append(args, bookID)
	}

	rows, err := pool.Query(ctx, `SELECT page.id, page.book_id, page.page_index,
		page.storage_key, page.original_ext, page.width, page.height, page.source_page_id,
		page.rolled_back, page.cut_side, page.cut_column, page.image_book_key
		FROM book_page page
		WHERE EXISTS (SELECT 1 FROM page_ocr WHERE page_id = page.id AND model = $1)
		AND EXISTS (SELECT 1 FROM page_ocr WHERE page_id = page.id AND model = ANY($2))`+
		where+` ORDER BY page.book_id, page.page_index`, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var pages []books.Page
	for rows.Next() {
		var page books.Page
		if err := rows.Scan(&page.ID, &page.BookID, &page.PageIndex, &page.StorageKey,
			&page.OriginalExt, &page.Width, &page.Height, &page.SourcePageID,
			&page.RolledBack, &page.CutSide, &page.CutColumn, &page.ImageBookKey); err != nil {
			return nil, err
		}
		pages = append(pages, page)
	}
	return pages, rows.Err()
}

// mergePage writes one merged reading of one page. The second answer is false for
// a page that holds no text of that engine, which is every page the other merge
// is for.
func mergePage(ctx context.Context, store *books.Store, page books.Page,
	model, textEngine string, dryRun bool) (ocrmerge.Counts, bool, error) {
	rows, err := store.PageOCRResults(ctx, page.ID)
	if err != nil {
		return ocrmerge.Counts{}, false, err
	}

	held := make(map[string]books.PageOCR, len(rows))
	for _, row := range rows {
		held[row.Model] = row
	}

	boxes, readBoxes := held[ocr.ModeV6]
	text, readText := held[textEngine]
	if !readBoxes || !readText {
		return ocrmerge.Counts{}, false, nil
	}

	merged, err := mergeOf(boxes, text, page, textEngine)
	if err != nil {
		return ocrmerge.Counts{}, false, err
	}
	if dryRun {
		return merged.Counts, true, nil
	}

	data, err := json.Marshal(merged)
	if err != nil {
		return ocrmerge.Counts{}, false, err
	}

	// No rank: this rewrites a reading the page already holds, so which reading
	// answers is not its to change.
	err = store.InsertPageOCR(ctx, books.PageOCR{
		PageID: page.ID,
		Model:  model,
		Text:   ocr.MergedText(merged),
		Data:   data,
	}, nil)
	if err != nil {
		return ocrmerge.Counts{}, false, err
	}
	return merged.Counts, true, nil
}

// mergeOf merges by the shape of the text engine's result. PaddleOCR-VL reports
// blocks with boxes, and GLM-5.3 Flash reports text with no geometry at all.
func mergeOf(boxes, text books.PageOCR, page books.Page,
	textEngine string) (ocrmerge.Result, error) {
	if textEngine != ocr.ModeGLM {
		return ocr.MergeOf(boxes.Data, text.Data)
	}

	merged, read, err := ocr.MergeOfText(boxes.Data, text.Text, page.Width, page.Height)
	if err != nil {
		return ocrmerge.Result{}, err
	}
	if !read {
		// Writing this would name a reading holding none of its own text. The row
		// the page already holds stays as it is, because a stale reading of the
		// page is worth more than one that is no reading of it.
		return ocrmerge.Result{}, fmt.Errorf(
			"the stored %s text places on %d of %d lines, too few to be a reading of the page",
			textEngine, merged.Counts.FromText, merged.Counts.Lines)
	}
	return merged, nil
}
