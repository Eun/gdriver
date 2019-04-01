package gdriver

import (
	"fmt"
)

// CallbackError will be returned if the callback returned an error
type CallbackError struct {
	NestedError error
}

func (e CallbackError) Error() string {
	return fmt.Sprintf("callback throwed an error: %v", e.NestedError)
}

// FileNotExistError will be thrown if an file was not found
type FileNotExistError struct {
	Path string
}

func (e FileNotExistError) Error() string {
	return fmt.Sprintf("`%s' does not exist", e.Path)
}

// FileExistError will be thrown if an file exists
type FileExistError struct {
	Path string
}

func (e FileExistError) Error() string {
	return fmt.Sprintf("`%s' already exists", e.Path)
}

// IsNotExist returns true if the error is an FileNotExistError
func IsNotExist(e error) bool {
	_, ok := e.(FileNotExistError)
	return ok
}

// IsExist returns true if the error is an FileExistError
func IsExist(e error) bool {
	_, ok := e.(FileExistError)
	return ok
}

// FileIsDirectoryError will be thrown if a file is a directory
type FileIsDirectoryError struct {
	Path string
}

func (e FileIsDirectoryError) Error() string {
	return fmt.Sprintf("`%s' is a directory", e.Path)
}

// FileIsNotDirectoryError will be thrown if a file is not a directory
type FileIsNotDirectoryError struct {
	Path string
}

func (e FileIsNotDirectoryError) Error() string {
	return fmt.Sprintf("`%s' is not a directory", e.Path)
}
