package lrucache

import "sync"

// action represents the type of operation or event to be processed in the cache.
type action uint8

// Enumeration of possible actions that can be performed on the cache.
const (
	EventActionAddToFront    action = iota // Add a node to the front of the list (most recently used).
	EventActionRemove                      // Remove a specific node from the cache.
	EventActionMakeSpaceFor                // Make space for a new entry by evicting older ones.
	EventActionRemoveExpired               // Remove all expired entries from the cache.
)

// event represents a specific operation to be performed on the cache.
// It is used in the asynchronous event channel for managing the linked list and cache state.
// Ordered to try and reduce padding.
type event[K comparable, V any] struct {
	finished *sync.WaitGroup // Optional wait group to signal completion of the event.
	n        *node[K, V]     // The node involved in the action, if applicable.
	a        action          // The type of action to be performed (e.g., add, remove, etc.).
}
