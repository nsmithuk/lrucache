package lrucache

import "errors"

var (
	ErrPastExpiry = errors.New("the expiry date cannot be in the past")
	ErrItemTooBig = errors.New("the item is too big to fit in the cache")
)
