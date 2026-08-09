package domain

import "errors"

var (
	ErrNotFound       = errors.New("avatar not found")
	ErrAlreadyDeleted = errors.New("avatar already deleted")
	ErrConflict       = errors.New("avatar conflict")
)
