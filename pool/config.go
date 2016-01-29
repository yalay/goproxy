package pool

import "errors"

var (
	ErrNoSession       = errors.New("session in pool but can't pick one.")
	ErrSessionNotFound = errors.New("session not found.")
)
