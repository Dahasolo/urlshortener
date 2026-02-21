package service

import "fmt"

// ErrURLAlreadyExists возникает при попытке добавить дублирующийся URL.
type ErrURLAlreadyExists struct {
	ExistingID  string
	OriginalURL string
}

func (e *ErrURLAlreadyExists) Error() string {
	return fmt.Sprintf("URL %q already exists with ID %q", e.OriginalURL, e.ExistingID)
}

func (e *ErrURLAlreadyExists) Is(target error) bool {
	_, ok := target.(*ErrURLAlreadyExists)
	return ok
}
