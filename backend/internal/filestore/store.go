package filestore

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"unicode"
)

const (
	RenditionOriginal = "orig"
	RenditionDisplay  = "1280"
	RenditionThumb    = "480"

	DisplaySize = 1280
	ThumbSize   = 480

	maxNameBytes = 128
)

type Store struct {
	root string
}

func New(root string) (*Store, error) {
	store := &Store{root: root}

	for _, dir := range []string{store.PagesRoot(), store.SourcesRoot(), store.TempRoot()} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return nil, fmt.Errorf("create %s: %w", dir, err)
		}
	}

	return store, nil
}

func (s *Store) Root() string        { return s.root }
func (s *Store) PagesRoot() string   { return filepath.Join(s.root, "pages") }
func (s *Store) SourcesRoot() string { return filepath.Join(s.root, "sources") }
func (s *Store) TempRoot() string    { return filepath.Join(s.root, "tmp") }

func (s *Store) PageDir(bookKey string) string {
	return filepath.Join(s.PagesRoot(), bookKey)
}

func (s *Store) SourceDir(bookKey string) string {
	return filepath.Join(s.SourcesRoot(), bookKey)
}

func (s *Store) PagePath(bookKey, pageKey, rendition, ext string) string {
	return filepath.Join(s.PageDir(bookKey), pageKey+"."+rendition+"."+ext)
}

func (s *Store) NewTempDir(uploadKey string) (string, error) {
	dir := filepath.Join(s.TempRoot(), uploadKey)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	return dir, nil
}

func PageURL(bookKey, pageKey, rendition, ext string) string {
	return "/files/" + bookKey + "/" + pageKey + "." + rendition + "." + ext
}

// SanitizeName makes an uploaded file name safe to join onto a path. The name
// reaches the filesystem, so separators and control characters cannot survive.
func SanitizeName(name string) string {
	name = strings.ReplaceAll(name, "\\", "/")
	if index := strings.LastIndex(name, "/"); index >= 0 {
		name = name[index+1:]
	}

	var builder strings.Builder
	for _, r := range name {
		switch {
		case unicode.IsControl(r):
		case r == 0:
		default:
			builder.WriteRune(r)
		}
	}

	cleaned := strings.Trim(builder.String(), " .")
	if len(cleaned) > maxNameBytes {
		cleaned = cleaned[:maxNameBytes]
		cleaned = strings.ToValidUTF8(cleaned, "")
	}

	if cleaned == "" {
		return "file"
	}
	return cleaned
}
