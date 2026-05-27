package keenbase

import (
	"encoding/json"
	"fmt"
	"image"
	_ "image/gif"
	"image/jpeg"
	_ "image/png"
	"io"
	"mime"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/disintegration/imaging"
)

const maxUploadMemory = 32 << 20 // 32 MB

type FileStore struct {
	dataDir string
}

func newFileStore(dataDir string) *FileStore {
	return &FileStore{dataDir: dataDir}
}

func (fs *FileStore) baseDir(collectionID, recordID string) string {
	return filepath.Join(fs.dataDir, "storage", collectionID, recordID)
}

func (fs *FileStore) FilePath(collectionID, recordID, filename string) string {
	return filepath.Join(fs.baseDir(collectionID, recordID), filename)
}

func (fs *FileStore) thumbPath(collectionID, recordID, size, filename string) string {
	return filepath.Join(fs.baseDir(collectionID, recordID), "thumbs", size, filename)
}

func (fs *FileStore) SaveFile(collectionID, recordID, fieldName, origFilename string, src io.Reader) (string, error) {
	ext := strings.ToLower(filepath.Ext(origFilename))
	storedName := fieldName + "_" + newID() + ext

	dir := fs.baseDir(collectionID, recordID)
	if err := os.MkdirAll(dir, 0750); err != nil {
		return "", fmt.Errorf("mkdir: %w", err)
	}

	dst, err := os.Create(filepath.Join(dir, storedName))
	if err != nil {
		return "", fmt.Errorf("create file: %w", err)
	}
	defer dst.Close()

	if _, err := io.Copy(dst, src); err != nil {
		return "", fmt.Errorf("write file: %w", err)
	}

	return storedName, nil
}

func (fs *FileStore) DeleteFile(collectionID, recordID, filename string) error {
	path := fs.FilePath(collectionID, recordID, filename)
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return err
	}

	thumbsDir := filepath.Join(fs.baseDir(collectionID, recordID), "thumbs")
	entries, _ := os.ReadDir(thumbsDir)
	for _, e := range entries {
		if e.IsDir() {
			tp := filepath.Join(thumbsDir, e.Name(), filename)
			os.Remove(tp) // best-effort
		}
	}
	return nil
}

func (fs *FileStore) DeleteRecordFiles(collectionID, recordID string) error {
	dir := fs.baseDir(collectionID, recordID)
	if err := os.RemoveAll(dir); err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}

func (fs *FileStore) ServeFile(w http.ResponseWriter, r *http.Request, collectionID, recordID, filename, thumb string) {
	if thumb != "" {
		fs.serveThumb(w, r, collectionID, recordID, filename, thumb)
		return
	}

	path := fs.FilePath(collectionID, recordID, filename)
	http.ServeFile(w, r, path)
}

func (fs *FileStore) serveThumb(w http.ResponseWriter, r *http.Request, collectionID, recordID, filename, size string) {
	w2, h2, err := parseThumbSize(size)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid thumb size")
		return
	}

	cachePath := fs.thumbPath(collectionID, recordID, size, filename)

	if _, err := os.Stat(cachePath); err == nil {
		http.ServeFile(w, r, cachePath)
		return
	}

	srcPath := fs.FilePath(collectionID, recordID, filename)
	srcFile, err := os.Open(srcPath)
	if err != nil {
		if os.IsNotExist(err) {
			http.NotFound(w, r)
			return
		}
		writeError(w, http.StatusInternalServerError, "could not open file")
		return
	}
	defer srcFile.Close()

	img, _, err := image.Decode(srcFile)
	if err != nil {
		// Not an image — just serve the original.
		http.ServeFile(w, r, srcPath)
		return
	}

	var resized *image.NRGBA
	switch {
	case w2 > 0 && h2 > 0:
		resized = imaging.Fill(img, w2, h2, imaging.Center, imaging.Lanczos)
	case w2 > 0:
		resized = imaging.Resize(img, w2, 0, imaging.Lanczos)
	default:
		resized = imaging.Resize(img, 0, h2, imaging.Lanczos)
	}

	if err := os.MkdirAll(filepath.Dir(cachePath), 0750); err == nil {
		if f, err := os.Create(cachePath); err == nil {
			jpeg.Encode(f, resized, &jpeg.Options{Quality: 80}) // best-effort
			f.Close()
		}
	}

	w.Header().Set("Content-Type", "image/jpeg")
	jpeg.Encode(w, resized, &jpeg.Options{Quality: 80})
}

func parseThumbSize(s string) (w, h int, err error) {
	parts := strings.SplitN(strings.ToLower(s), "x", 2)
	if len(parts) != 2 {
		return 0, 0, fmt.Errorf("invalid format")
	}
	if parts[0] != "" && parts[0] != "0" {
		w, err = strconv.Atoi(parts[0])
		if err != nil {
			return 0, 0, err
		}
	}
	if parts[1] != "" && parts[1] != "0" {
		h, err = strconv.Atoi(parts[1])
		if err != nil {
			return 0, 0, err
		}
	}
	if w == 0 && h == 0 {
		return 0, 0, fmt.Errorf("width and height cannot both be zero")
	}
	return w, h, nil
}

func (sb *SimpleBase) parseRecordBody(r *http.Request, col *Collection) (map[string]any, error) {
	ct, _, _ := mime.ParseMediaType(r.Header.Get("Content-Type"))

	if ct != "multipart/form-data" {
		// Plain JSON body — existing behaviour.
		var data map[string]any
		if err := json.NewDecoder(r.Body).Decode(&data); err != nil {
			return nil, fmt.Errorf("invalid request body")
		}
		return data, nil
	}

	if err := r.ParseMultipartForm(maxUploadMemory); err != nil {
		return nil, fmt.Errorf("could not parse multipart form: %w", err)
	}

	data := make(map[string]any)

	for _, f := range col.Fields {
		if f.Type == FieldTypeFile {
			continue
		}
		vals, ok := r.MultipartForm.Value[f.Name]
		if !ok || len(vals) == 0 {
			continue
		}
		data[f.Name] = coerceStringToField(f, vals[0])
	}

	recordID := r.PathValue("id") // empty on create — we use the newID generated later
	if recordID == "" {
		// For create we don't have the record ID yet — generate a provisional
		// one and store it so the record store can use it.
		// We'll pass it in as the special "__id" key.
		recordID = newID()
		data["__id"] = recordID
	}

	for _, f := range col.Fields {
		if f.Type != FieldTypeFile {
			continue
		}

		fileHeaders, ok := r.MultipartForm.File[f.Name]
		if !ok || len(fileHeaders) == 0 {
			continue
		}

		maxSelect := 1
		if f.Options.MaxSelect != nil && *f.Options.MaxSelect > 1 {
			maxSelect = *f.Options.MaxSelect
		}

		var names []string
		for i, fh := range fileHeaders {
			if i >= maxSelect {
				break
			}
			src, err := fh.Open()
			if err != nil {
				return nil, fmt.Errorf("open uploaded file: %w", err)
			}
			storedName, err := sb.files.SaveFile(col.ID, recordID, f.Name, fh.Filename, src)
			src.Close()
			if err != nil {
				return nil, fmt.Errorf("save file: %w", err)
			}
			names = append(names, storedName)
		}

		if maxSelect == 1 {
			data[f.Name] = names[0]
		} else {
			// Store as JSON array.
			b, _ := json.Marshal(names)
			data[f.Name] = string(b)
		}
	}

	return data, nil
}

func coerceStringToField(f Field, s string) any {
	switch f.Type {
	case FieldTypeNumber:
		if v, err := strconv.ParseFloat(s, 64); err == nil {
			return v
		}
		return s
	case FieldTypeBool:
		return s == "true" || s == "1"
	case FieldTypeJSON:
		var v any
		if err := json.Unmarshal([]byte(s), &v); err == nil {
			return v
		}
		return s
	default:
		return s
	}
}
