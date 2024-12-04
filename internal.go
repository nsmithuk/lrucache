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
			lru.events <- event[K, V]{a: EventActionRemoveExpired, finished: wg}
			wg.Wait()

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

func (n *node[K, V]) flagAsDeleted() {
	n.deleted = true
}

// processEvents processes all events sent to the cache's event channel.
// This method handles all modifications to the linked list without requiring additional locks.
func (lru *Cache[K, V]) processEvents() {
	for e := range lru.events {
		switch e.a {
		case EventActionRemove:
			lru.lock.AssertLocked()

			// Remove a node from the cache.
			// Assumes the lock is already acquired.
			delete(lru.cache, e.n.key)
			lru.removeNodeFromList(e.n)
			lru.size -= e.n.size
			e.n.flagAsDeleted()

		case EventActionAddToFront:
			// Move a node to the front of the list (most recently used).

			// Validate that it's not been removed since being added to the buffer.
			if !e.n.deleted {
				lru.addNodeToHead(e.n)
			}

		case EventActionMakeSpaceFor:
			// Free up space in the cache for a new entry.
			// Assumes the lock is already acquired.
			spaceAvailable := lru.capacity - lru.size
			for spaceAvailable < e.n.size {
				lru.lock.AssertLocked()

				removed := lru.removeNodeFromTail()
				delete(lru.cache, removed.key)

				lru.size -= removed.size
				spaceAvailable = lru.capacity - lru.size
				removed.flagAsDeleted()
			}

		case EventActionRemoveExpired:
			// Remove all expired entries from the cache.
			// Assumes the lock is already acquired.
			now := time.Now()
			for _, n := range lru.cache {
				if !n.expires.IsZero() && n.expires.Before(now) {
					lru.lock.AssertLocked()

					delete(lru.cache, n.key)
					lru.removeNodeFromList(n)
					lru.size -= n.size
					n.flagAsDeleted()
				}
			}

		default:
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
