package lrucache

import "errors"

var (
	ErrPastExpiry   = errors.New("the expiry date cannot be in the past")
	ErrItemTooSmall = errors.New("the item size much the greater than or equal to 1")
	ErrItemTooBig   = errors.New("the item is too big to fit in the cache")
)
