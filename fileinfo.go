package gdriver

import (
	"fmt"
	"path"
	"time"

	drive "google.golang.org/api/drive/v3"
)

// FileInfo represents file information for a file or directory
type FileInfo struct {
	item       *drive.File
	parentPath string
}

// Name returns the name of the file or directory
func (i *FileInfo) Name() string {
	return sanitizeName(i.item.Name)
}

// ParentPath returns the parent path of the file or directory
func (i *FileInfo) ParentPath() string {
	return i.parentPath
}

// Path returns the full path to this file or directory
func (i *FileInfo) Path() string {
	return path.Join(i.parentPath, i.Name())
}

// Size returns the bytes for this file
func (i *FileInfo) Size() int64 {
	return i.item.Size
}

// CreationTime returns the time when this file was created
func (i *FileInfo) CreationTime() time.Time {
	t, err := time.Parse(time.RFC3339, i.item.CreatedTime)
	if err != nil {
		panic(fmt.Errorf("unable to parse CreatedTime (`%s'): %v", i.item.CreatedTime, err))
	}
	return t
}

// ModifiedTime returns the time when this file was modified
func (i *FileInfo) ModifiedTime() time.Time {
	t, err := time.Parse(time.RFC3339, i.item.ModifiedTime)
	if err != nil {
		panic(fmt.Errorf("unable to parse ModifiedTime (`%s'): %v", i.item.ModifiedTime, err))
	}
	return t
}

// IsDir returns true if this file is a directory
func (i *FileInfo) IsDir() bool {
	return i.item.MimeType == mimeTypeFolder
}

// DriveFile returns the underlaying drive.File
func (i *FileInfo) DriveFile() *drive.File {
	return i.item
}

func sanitizeName(s string) string {
	runes := []rune(s)
	for i, r := range runes {
		if isPathSeperator(r) || r == '\'' {
			runes[i] = '-'
		}
	}
	return string(runes)
}

func isPathSeperator(r rune) bool {
	return r == '/' || r == '\\'
}
