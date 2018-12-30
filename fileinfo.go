package gdriver

import (
	"fmt"
	"os"
	"time"

	drive "google.golang.org/api/drive/v3"
)

type FileInfo struct {
	sys *drive.File
}

func (i *FileInfo) Name() string {
	return sanitizeName(i.sys.Name)
}
func (i *FileInfo) Size() int64 {
	return i.sys.Size
}
func (i *FileInfo) Mode() os.FileMode {
	return 0700
}
func (i *FileInfo) CreationTime() time.Time {
	t, err := time.Parse(time.RFC3339, i.sys.CreatedTime)
	if err != nil {
		panic(fmt.Errorf("unable to parse CreatedTime (`%s'): %v", i.sys.CreatedTime, err))
	}
	return t
}
func (i *FileInfo) ModTime() time.Time {
	t, err := time.Parse(time.RFC3339, i.sys.ModifiedTime)
	if err != nil {
		panic(fmt.Errorf("unable to parse ModifiedTime (`%s'): %v", i.sys.ModifiedTime, err))
	}
	return t
}
func (i *FileInfo) IsDir() bool {
	return i.sys.MimeType == mimeTypeFolder
}
func (i *FileInfo) Sys() interface{} {
	return i.sys
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
