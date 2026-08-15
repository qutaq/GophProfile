package domain

import "errors"

var (
	ErrNotFound          = errors.New("avatar not found")
	ErrAlreadyDeleted    = errors.New("avatar already deleted")
	ErrConflict          = errors.New("avatar conflict")
	ErrForbidden         = errors.New("forbidden")
	ErrMissingUserID     = errors.New("missing user id")
	ErrInvalidFile       = errors.New("invalid file format")
	ErrFileTooLarge      = errors.New("file too large")
	ErrEmptyFile         = errors.New("empty file")
	ErrThumbnailNotReady = errors.New("thumbnail not ready")
	ErrInvalidSize       = errors.New("invalid size")
)
