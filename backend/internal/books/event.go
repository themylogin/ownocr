package books

import "github.com/themylogin/ownocr/backend/internal/filestore"

// Event mirrors the book summary the REST API returns, so a client can apply an
// event and a fetch through the same code path.
func Event(book Book, cover *Page) map[string]any {
	event := map[string]any{
		"id":           book.ID,
		"name":         book.Name,
		"state":        book.State,
		"stage":        book.Stage,
		"pageCount":    book.PageCount,
		"error":        book.Error,
		"coverUrl":     nil,
		"createdAt":    book.CreatedAt.UTC().Format("2006-01-02T15:04:05.999999Z07:00"),
		"derivedState": book.DerivedState,
		"progress":     book.Progress,
		"action":       book.Action,
		"ocrProfile":   book.OCRProfile,
		"sourceBookId": book.SourceBookID,
	}

	if cover != nil {
		event["coverUrl"] = filestore.PageURL(cover.ImageKey(book.StorageKey), cover.StorageKey,
			filestore.RenditionThumb, "jpg")
	}

	return event
}
