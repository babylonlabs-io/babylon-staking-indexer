package db

import (
	"errors"
	"fmt"
)

// DuplicateKeyError is an error type for duplicate key errors
type DuplicateKeyError struct {
	Key     string
	Message string
}

func (e *DuplicateKeyError) Error() string {
	return e.Message
}

func (e *DuplicateKeyError) Is(target error) bool {
	_, ok := target.(*DuplicateKeyError)
	return ok
}

func IsDuplicateKeyError(err error) bool {
	return errors.Is(err, &DuplicateKeyError{})
}

// InvalidPaginationTokenError is an error type for invalid pagination token errors
type InvalidPaginationTokenError struct {
	Message string
}

func (e *InvalidPaginationTokenError) Error() string {
	return e.Message
}

func (e *InvalidPaginationTokenError) Is(target error) bool {
	_, ok := target.(*InvalidPaginationTokenError)
	return ok
}

func IsInvalidPaginationTokenError(err error) bool {
	return errors.Is(err, &InvalidPaginationTokenError{})
}

// Not found Error
type NotFoundError struct {
	Key     string
	Message string
}

func (e *NotFoundError) Error() string {
	return fmt.Sprintf("%s: %s", e.Message, e.Key)
}

func (e *NotFoundError) Is(target error) bool {
	_, ok := target.(*NotFoundError)
	return ok
}

func IsNotFoundError(err error) bool {
	return errors.Is(err, &NotFoundError{})
}
