package lrucache

import (
	"sync"
	"time"
)

// purgeExpired periodically checks and removes expired entries from the cache.
// - dur: The duration between successive checks for expired entries.
func (lru *Cache[K, V]) purgeExpired(dur time.Duration) {
	for {
		select {
		case <-lru.done:
			// Exit the loop when the cache is closed.
			return
		case <-time.After(dur):
			// Triggered at regular intervals.
			lru.lock.Lock()

			// Send an event to remove expired entries.
			wg := &sync.WaitGroup{}
			wg.Add(1)
			lru.events <- event[K, V]{a: EventActionRemoveExpired, n: nil, finished: wg}
			wg.Wait() // Wait for the operation to complete.

			lru.lock.Unlock()
		}
	}
}

// deleteNode removes a node from the cache and processes it for cleanup.
// Assumes the lock is already acquired.
func (lru *Cache[K, V]) deleteNode(n *node[K, V]) {
	wg := &sync.WaitGroup{}
	wg.Add(1)

	// Send an event to remove the node.
	lru.events <- event[K, V]{a: EventActionRemove, n: n, finished: wg}
	wg.Wait() // Wait for the node removal to complete.
}

// returnToPool resets a node's fields and returns it to the pool for reuse.
// - n: The node to be returned to the pool.
func (lru *Cache[K, V]) returnToPool(n *node[K, V]) {
	// Reset the node's fields to avoid retaining references.
	n.key = lru.emptyK
	n.value = lru.emptyV
	n.previous = nil
	n.next = nil

	// Put the node back into the pool.
	lru.pool.Put(n)
}

// processEvents processes all events sent to the cache's event channel.
// This method handles all modifications to the linked list without requiring additional locks.
func (lru *Cache[K, V]) processEvents() {
	for e := range lru.events {
		switch e.a {
		case EventActionRemove:
			// Remove a node from the cache.
			// Assumes the lock is already acquired.
			delete(lru.cache, e.n.key)
			lru.removeNodeFromList(e.n)
			lru.size -= e.n.size
			lru.returnToPool(e.n)

		case EventActionAddToFront:
			// Move a node to the front of the list (most recently used).
			lru.addNodeToHead(e.n)

		case EventActionMakeSpaceFor:
			// Free up space in the cache for a new entry.
			// Assumes the lock is already acquired.
			spaceAvailable := lru.capacity - lru.size
			for spaceAvailable < e.n.size {
				removed := lru.removeNodeFromTail()
				delete(lru.cache, removed.key)

				// Update the cache size and space available.
				lru.size -= removed.size
				spaceAvailable = lru.capacity - lru.size
				lru.returnToPool(removed)
			}

		case EventActionRemoveExpired:
			// Remove all expired entries from the cache.
			// Assumes the lock is already acquired.
			for k, n := range lru.cache {
				if !n.expires.IsZero() && n.expires.Before(time.Now()) {
					delete(lru.cache, k)
					lru.removeNodeFromList(n)
					lru.size -= n.size
					lru.returnToPool(n)
				}
			}

		default:
			// Panic if an unknown event type is received.
			panic("unknown action")
		}

		// Signal that the event has been processed, if a wait group is provided.
		if e.finished != nil {
			e.finished.Done()
		}
	}
}

// removeNodeFromTail removes and returns the least recently used node (at the tail of the list).
func (lru *Cache[K, V]) removeNodeFromTail() *node[K, V] {
	last := lru.tail.previous
	lru.removeNodeFromList(last)
	return last
}

// removeNodeFromList removes a node from its current position in the doubly linked list.
// - n: The node to be removed.
func (lru *Cache[K, V]) removeNodeFromList(n *node[K, V]) {
	// Do nothing if the node is not part of the list.
	if n.next == nil || n.previous == nil {
		return
	}

	// Update pointers of adjacent nodes to bypass the node.
	n.previous.next = n.next
	n.next.previous = n.previous
}

// addNodeToHead moves a node to the head of the list (most recently used).
// If the node is already in the list, it removes it first.
func (lru *Cache[K, V]) addNodeToHead(n *node[K, V]) {
	// If the node is already in the list, remove it first.
	if n.previous != nil {
		lru.removeNodeFromList(n)
	}

	// Insert the node between the head and the current first node.
	lru.addNodeBetween(n, lru.head, lru.head.next)
}

// addNodeBetween inserts a node between two given nodes in the list.
// - n: The node to be inserted.
// - previous: The node that will precede the new node.
// - next: The node that will follow the new node.
func (lru *Cache[K, V]) addNodeBetween(n, previous, next *node[K, V]) {
	// Update pointers to insert the new node.
	previous.next = n
	next.previous = n
	n.previous = previous
	n.next = next
}
