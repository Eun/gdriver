package gdriver

import (
	"errors"
	"io"
	"sync"
)

type File interface {
	Info() *FileInfo
	Write([]byte) (int, error)
	Read([]byte) (int, error)
	Close() error
}

type readFile struct {
	Driver *GDriver
	*FileInfo
	reader io.ReadCloser
	once   sync.Once
}

func (f *readFile) Info() *FileInfo {
	return f.FileInfo
}

func (f *readFile) getReader() error {
	var lastErr error
	f.once.Do(func() {
		response, err := f.Driver.srv.Files.Get(f.item.Id).Download()
		if err != nil {
			lastErr = err
			return
		}
		f.reader = response.Body
	})
	return lastErr
}

func (f *readFile) Write(p []byte) (int, error) {
	return 0, errors.New("open the file with O_WRONLY for writing")
}

func (f *readFile) Read(p []byte) (int, error) {
	if err := f.getReader(); err != nil {
		return 0, err
	}
	return f.reader.Read(p)
}

func (f *readFile) Close() error {
	if err := f.getReader(); err != nil {
		return err
	}
	return f.reader.Close()
}

type writeFile struct {
	Driver *GDriver
	Path   string
	*FileInfo
	writer   *io.PipeWriter
	mu       sync.Mutex
	doneChan chan struct{}
	putError error
}

func (f *writeFile) Info() *FileInfo {
	return f.FileInfo
}

func (f *writeFile) getWriter() error {
	f.mu.Lock()
	if f.doneChan == nil {
		var reader io.Reader
		// open a pipe and use the writer part for Write()
		reader, f.writer = io.Pipe()
		// the channel is used to notify the Close() or Write() function if something goes wrong
		f.doneChan = make(chan struct{})
		go func() {
			if f.FileInfo == nil {
				f.FileInfo, f.putError = f.Driver.PutFile(f.Path, reader)
			} else {
				f.putError = f.Driver.updateFileContents(f.FileInfo.item.Id, reader)
			}
			f.doneChan <- struct{}{}
		}()
	}
	err := f.putError
	f.mu.Unlock()
	return err
}

func (f *writeFile) Write(p []byte) (int, error) {
	if err := f.getWriter(); err != nil {
		return 0, err
	}
	return f.writer.Write(p)
}

func (f *writeFile) Read(p []byte) (int, error) {
	return 0, errors.New("open the file with O_RDONLY for writing")
}

func (f *writeFile) Close() error {
	closeErr := f.writer.Close()
	if f.doneChan != nil {
		<-f.doneChan
		if err := f.putError; err != nil {
			return err
		}
	}
	return closeErr
}
