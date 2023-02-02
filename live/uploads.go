package live

import (
	"context"
	"fmt"
	"mime"
	"os"
	"path/filepath"

	"github.com/google/uuid"
)

// A file and related metadata selected for upload
type UploadEntry struct {

	// Whether the file selection has been cancelled. Defaults to false.
	Cancelled bool

	// The timestamp when the file was last modified from the client's file system
	LastModified int64

	// The name of the file from the client's file system
	Name string

	// The size of the file in bytes from the client's file system
	Size int64

	// The mime type of the file from the client's file system
	Type string

	// True if the file has been uploaded. Defaults to false.
	Done bool

	// True if the file has been auto-uploaded. Defaults to false.
	Preflighted bool

	// The integer percentage of the file that has been uploaded. Defaults to 0.
	Progress int

	// The unique instance ref of the upload entry
	Ref string

	// The unique instance ref of the upload config to which this entry belongs
	UploadRef string

	// A uuid for the file
	UUID string //uuid.UUID

	// True if there are no errors with the file. Defaults to true.
	Valid bool

	// Errors that have occurred during selection or upload.
	Errors []string
}

// UploadConfig is the configuration and entry related details for uploading files.
type UploadConfig struct {
	// Name is the unique of the upload config referenced in the LiveView
	Name string

	// Entries are the set of UploadEntries selected for upload
	Entries []UploadEntry

	// Ref is the unique instance ref of the upload config
	Ref string

	// Errors contains the set of errors that have occurred during selection or upload.
	Errors []string

	// AutoUpload determines whether to upload the selected files automatically when selected on the client. Defaults to false.
	AutoUpload bool

	UploadConstraints
}

// validateType checks if the file type is allowed by the upload config
func (uc *UploadConfig) validateType(mimeType string) bool {
	for _, t := range uc.Accept {
		// we don't know if Accept is a file extension or a mime type
		// so we'll try to match both
		if t == mimeType {
			return true
		}
		exts, _ := mime.ExtensionsByType(mimeType)
		// ignore errors if we can't find the extension
		for _, ext := range exts {
			if t == ext {
				return true
			}
		}
	}
	return false
}

// validateSize checks if the file size is allowed by the upload config
func (uc *UploadConfig) validateSize(size int64) bool {
	return size <= uc.MaxFileSize
}

// validateEntry checks if the entry passes the upload config's validation rules
// and sets the entry's Error slice accordingly
func (uc *UploadConfig) validateEntry(entry *UploadEntry) bool {
	if entry.Cancelled {
		return false
	}
	if !uc.validateSize(entry.Size) {
		entry.Errors = append(entry.Errors, fmt.Sprintf("file size exceeds max of %d", uc.MaxFileSize))
		return false
	}
	if !uc.validateType(entry.Type) {
		entry.Errors = append(entry.Errors, fmt.Sprintf("file type %s is not allowed", entry.Type))
		return false
	}
	return true
}

// AddEnties adds all the entries to the upload config and validates each entry and the config as a whole
func (uc *UploadConfig) AddEntries(entries []any) {
	es := make([]UploadEntry, len(entries))
	for i, me := range entries {
		e := me.(map[string]any)

		entry := UploadEntry{
			Cancelled:    false,
			LastModified: int64(e["last_modified"].(float64)),
			Name:         e["name"].(string),
			Size:         int64(e["size"].(float64)),
			Type:         e["type"].(string),
			Done:         false,
			Preflighted:  false,
			Progress:     0,
			Ref:          e["ref"].(string),
			UploadRef:    uc.Ref,
			UUID:         uuid.NewString(),
			Errors:       []string{},
		}
		entry.Valid = uc.validateEntry(&entry)

		es[i] = entry
	}
	if uc.MaxEntries < len(es) {
		uc.Errors = append(uc.Errors, fmt.Sprintf("max entries exceeded: %d", uc.MaxEntries))
	}
	uc.Entries = es
}

// RemoveEntry removes an entry from the upload config for given ref
func (uc *UploadConfig) RemoveEntry(ref string) {
	for i, e := range uc.Entries {
		if e.Ref == ref {
			// add elements before and after the element to remove
			uc.Entries = append(uc.Entries[:i], uc.Entries[i+1:]...)
			break
		}
	}
}

// UploadConstraints contains the file constraints for the upload config
type UploadConstraints struct {
	// Accept is the slice of unique file type specifiers that can be uploaded.
	// See https://developer.mozilla.org/en-US/docs/Web/HTML/Element/input/file#unique_file_type_specifiers
	Accept []string

	// MaxEntries is the maximum number of files that can be uploaded at once. Defaults to 10.
	MaxEntries int `json:"max_entries"`

	// MaxFileSize is maximum size of each file in bytes. Defaults to 10MB.
	MaxFileSize int64 `json:"max_file_size"`

	// ChunkSize is the size of each chunk of an uploaded file in bytes. Defaults to 64kb.
	ChunkSize int64 `json:"chunk_size"`
}

// NewUploadConstraints returns a new UploadConstraints merged with the default values
func NewUploadConstraints(uc *UploadConfig) UploadConstraints {
	us := UploadConstraints{
		ChunkSize:   64 * 1024, // 64kb
		MaxEntries:  10,
		MaxFileSize: 10 * 1024 * 1024, // 10MB
	}
	if uc.ChunkSize > 0 {
		us.ChunkSize = uc.ChunkSize
	}
	if uc.MaxEntries > 0 {
		us.MaxEntries = uc.MaxEntries
	}
	if uc.MaxFileSize > 0 {
		us.MaxFileSize = uc.MaxFileSize
	}
	return us
}

type ConsumeUploadedEntriesMeta struct {
	//  The location of the file on the server
	Path string
}

// Allows file uploads for the given `LiveView`and configures the upload
// options (filetypes, size, etc).
func AllowUpload(ctx context.Context, name string, options UploadConstraints) error {
	uc := &UploadConfig{
		Ref:               "phx-" + uuid.New().String(),
		Name:              name,
		UploadConstraints: options,
	}
	s := socketValue(ctx)
	if s == nil {
		return nil
	}
	s.uploadConfigs[name] = uc
	return nil
}

// Cancels the file upload for a given UploadConfig by config name and file ref.
func CancelUpload(ctx context.Context, configName string, ref string) error {
	s := socketValue(ctx)
	if s == nil {
		return nil
	}
	uc := s.uploadConfigs[configName]
	if uc == nil {
		return nil
	}
	uc.RemoveEntry(ref)
	return nil
}

// Consume the uploaded files for a given UploadConfig (by name). This
// should only be called after the form's "save" event has occurred which
// guarantees all the files for the upload have been fully uploaded.
func ConsumeUploadedEntries(
	ctx context.Context,
	configName string,
	fn func(meta ConsumeUploadedEntriesMeta, entry UploadEntry) any,
) []any {
	s := socketValue(ctx)
	if s == nil {
		return nil
	}
	uc := s.uploadConfigs[configName]
	if uc == nil {
		return nil
	}
	var res []any
	tdir := filepath.Join(os.TempDir(), fmt.Sprintf("golive-%s", s.activeUploadRef))
	for _, entry := range uc.Entries {
		if !entry.Done {
			panic("cannot consume entries that are not fully uploaded")
		}
		path := filepath.Join(tdir, entry.UUID)
		res = append(res, fn(ConsumeUploadedEntriesMeta{Path: path}, entry))
	}
	// clear the entries
	uc.Entries = make([]UploadEntry, 0)
	return res
}

// Returns two sets of files that are being uploaded, those `completed` and
// those `inProgress` for a given UploadConfig (by name).  Unlike `consumeUploadedEntries`,
// this does not require the form's "save" event to have occurred and will not
// throw if any of the entries are not fully uploaded.
func UploadedEntries(ctx context.Context, configName string) (completed []UploadEntry, inProgress []UploadEntry) {
	s := socketValue(ctx)
	if s == nil {
		return nil, nil
	}
	uc := s.uploadConfigs[configName]
	if uc == nil {
		return nil, nil
	}

	for _, entry := range uc.Entries {
		if entry.Done {
			completed = append(completed, entry)
		} else {
			inProgress = append(inProgress, entry)
		}
	}
	return completed, inProgress
}

// TODO use this instead of pulling from map[string]any in AddEntries
// type AllowUploadEntry struct {
// 	LastModified int64  `json:"last_modified"`
// 	Name         string `json:"name"`
// 	Size         int64  `json:"size"`
// 	Type         string `json:"type"`
// 	Ref          string `json:"ref"`
// }
