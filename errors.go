package gdriver

import "fmt"

// CallbackError will be returned if the callback returned an error
type CallbackError struct {
	NestedError error
}

func (e CallbackError) Error() string {
	return fmt.Sprintf("callback throwed an error: %v", e.NestedError)
}

// NotFoundError will be thown if an file was not found
type NotFoundError struct {
	Path string
}

func (e NotFoundError) Error() string {
	return fmt.Sprintf("`%s' not found", e.Path)
}
