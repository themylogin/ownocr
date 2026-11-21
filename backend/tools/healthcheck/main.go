// Command healthcheck runs the disagreement rule over the books a database
// holds and reports the pages that disagree. It reads and writes nothing, so it
// is safe against the development database.
//
// It exists to hold the rule against real data. A page it names is a page a
// reader should look at, and the faults say why.
package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"sort"
	"strconv"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/themylogin/ownocr/backend/internal/books"
	"github.com/themylogin/ownocr/backend/internal/health"
	"github.com/themylogin/ownocr/backend/internal/ocr"
)

func main() {
	database := flag.String("database", os.Getenv("DATABASE_URL"), "database URL")
	ids := flag.String("books", "", "comma separated book ids, empty for every read book")
	base := flag.String("base", "https://ownocr.themylogin.com", "base URL of the reader links")
	flag.Parse()

	if err := run(context.Background(), *database, *ids, *base); err != nil {
		fmt.Fprintln(os.Stderr, "healthcheck:", err)
		os.Exit(1)
	}
}

func run(ctx context.Context, database, ids, base string) error {
	if database == "" {
		return fmt.Errorf("no database URL: pass -database or set DATABASE_URL")
	}

	pool, err := pgxpool.New(ctx, database)
	if err != nil {
		return err
	}
	defer pool.Close()

	store := books.NewStore(pool)

	chosen, err := chosenBooks(ctx, pool, ids)
	if err != nil {
		return err
	}

	total := 0
	for _, bookID := range chosen {
		flagged, judged, err := judgeBook(ctx, store, bookID, base)
		if err != nil {
			return err
		}
		if judged == 0 {
			continue
		}
		fmt.Printf("book %d: %d of %d pages disagree\n", bookID, flagged, judged)
		total += flagged
	}

	fmt.Printf("\n%d pages disagree over %d books\n", total, len(chosen))
	return nil
}

func judgeBook(ctx context.Context, store *books.Store, bookID int64,
	base string) (int, int, error) {
	pages, err := store.Pages(ctx, bookID)
	if err != nil {
		return 0, 0, err
	}

	answering, err := store.BookReadings(ctx, bookID, ocr.RenderModels)
	if err != nil {
		return 0, 0, err
	}

	lines, err := store.BookOCRIn(ctx, bookID, []string{ocr.ModeV6})
	if err != nil {
		return 0, 0, err
	}

	toJudge := make(map[int64]health.Page, len(pages))
	for _, page := range pages {
		answered, held := answering[page.ID]
		if !held {
			continue
		}

		text := answered.Text
		if source, merged := ocr.TextEngineOf(answered.Model); merged {
			engine, held, err := store.PageOCRIn(ctx, page.ID, []string{source})
			if err != nil {
				return 0, 0, err
			}
			if held {
				text = ocr.ReadingText(source, engine.Text, engine.Data)
			}
		}

		toJudge[page.ID] = health.Page{
			Answered:   text,
			Line:       lines[page.ID].Text,
			SameEngine: answered.Model == ocr.ModeV6,
		}
	}

	verdicts := health.Judgements(toJudge)

	sort.Slice(pages, func(i, j int) bool { return pages[i].PageIndex < pages[j].PageIndex })
	for _, page := range pages {
		verdict := verdicts[page.ID]
		if !verdict.Disagrees {
			continue
		}
		detail := fmt.Sprintf(" repeat=%.2f line=%.2f", verdict.Repeat, verdict.LineRepeat)
		for _, name := range verdict.Foreign {
			detail += fmt.Sprintf(" %s=%d", name, verdict.Counts[name])
		}
		fmt.Printf("  %s/books/%d/pages/%-4d %-26s%s\n", base, bookID, page.PageIndex,
			strings.Join(verdict.Faults, ", "), detail)
	}

	return health.Disagreeing(verdicts), len(toJudge), nil
}

func chosenBooks(ctx context.Context, pool *pgxpool.Pool, ids string) ([]int64, error) {
	if ids != "" {
		var chosen []int64
		for _, raw := range strings.Split(ids, ",") {
			id, err := strconv.ParseInt(strings.TrimSpace(raw), 10, 64)
			if err != nil {
				return nil, fmt.Errorf("book id %q: %w", raw, err)
			}
			chosen = append(chosen, id)
		}
		return chosen, nil
	}

	rows, err := pool.Query(ctx, `SELECT DISTINCT book_page.book_id
		FROM book_page JOIN page_ocr ON page_ocr.page_id = book_page.id
		ORDER BY book_page.book_id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var chosen []int64
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		chosen = append(chosen, id)
	}
	return chosen, rows.Err()
}
