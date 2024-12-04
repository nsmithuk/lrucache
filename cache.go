package lrucache

import (
	"fmt"
	"sync"
	"time"
)

// DateTime is the format used for logging and error messages.
const (
	DateTime = "2006-01-02 15:04:05.000 MST"
)

// Default configuration values for the LRU cache.
const (
	// DefaultPurgeTimerInterval By default we'll disable purging expired nodes on a regular interval.
	// i.e. nodes will only be removed when pushed beyond the tail, or purged if `PurgeExpiredEventsWhenCacheIsFull` is true.
	DefaultPurgeTimerInterval = 0

	// DefaultBufferSize zero essentially means no buffer, thus everything is strongly consistent.
	// Any other value will result in the list-ordering being eventually consistent when a Get() is called.
	// Note that Set() and Delete() calls are always strongly consistent.
	DefaultBufferSize = 0

	DefaultPurgeExpiredEventsWhenCacheIsFull = false
)

var (
	// PurgeExpiredEventsWhenCacheIsFull if true, when the cache's capacity is reached, we will try and purge expired
	// nodes before we start removing nodes from the tail.
	// Benefit: If we're able to clear enough space from expired nodes, we won't have to remove tail nodes (which *might* not have expired).
	// Cost: The purge operation requires a full lock, and has a O(n) time-complexity. This will likely slow the Set() operations down, and block the rest of the cache.
	PurgeExpiredEventsWhenCacheIsFull = DefaultPurgeExpiredEventsWhenCacheIsFull
)

// Cache represents a thread-safe, generic LRU (Least Recently Used) cache.
// K is the type of the keys (must be comparable), and V is the type of the values.
type Cache[K comparable, V any] struct {
	size     uint64 // Current total size of all items in the cache.
	capacity uint64 // Maximum allowed size of the cache.

	cache map[K]*node[K, V] // Map for fast key-based lookup of nodes.

	head *node[K, V] // Pointer to the most recently used node.
	tail *node[K, V] // Pointer to the least recently used node.

	lock   AssertRWLock     // Lock for synchronising read/write operations.
	events chan event[K, V] // Channel for handling cache events asynchronously.
	done   chan bool        // Channel for signalling cache shutdown.
	close  sync.Once        // Ensures Close method runs only once.

	purgeInterval time.Duration

	emptyK K // Zero value for the key type, used for default returns.
	emptyV V // Zero value for the value type, used for default returns.
}

// node represents an individual entry in the LRU cache.
// Ordered to try and reduce padding.
type node[K comparable, V any] struct {
	expires  time.Time   // Expiry time of the entry; zero value means no expiry.
	size     uint64      // Size of the entry in the cache.
	previous *node[K, V] // Pointer to the previous node in the linked list.
	next     *node[K, V] // Pointer to the next node in the linked list.
	key      K           // Key associated with the cache entry.
	value    V           // Value stored in the cache entry.
	deleted  bool
}

func NewCache[K comparable, V any](capacity uint64) *Cache[K, V] {
	return NewCacheWithBufferAndInterval[K, V](capacity, DefaultBufferSize, DefaultPurgeTimerInterval)
}

func NewCacheWithBuffer[K comparable, V any](capacity uint64, buffer uint16) *Cache[K, V] {
	return NewCacheWithBufferAndInterval[K, V](capacity, buffer, DefaultPurgeTimerInterval)
}

func NewCacheWithInterval[K comparable, V any](capacity uint64, interval time.Duration) *Cache[K, V] {
	return NewCacheWithBufferAndInterval[K, V](capacity, DefaultBufferSize, interval)
}

// NewCacheWithBufferAndInterval creates a new LRU cache with the specified capacity and event buffer size.
// - capacity: Maximum size of the cache.
// - buffer: Buffer size for the event channel.
// - buffer: Duration between purging expired nodes.
func NewCacheWithBufferAndInterval[K comparable, V any](capacity uint64, buffer uint16, interval time.Duration) *Cache[K, V] {
	cache := &Cache[K, V]{
		capacity: capacity,
		cache:    make(map[K]*node[K, V]),

		head: &node[K, V]{},
		tail: &node[K, V]{},

		done:   make(chan bool),
		events: make(chan event[K, V], buffer),

		purgeInterval: interval,
	}

	// Initialise the linked list with the head and tail nodes.
	cache.head.next = cache.tail
	cache.tail.previous = cache.head

	// Start background goroutines for processing events and purging expired items.
	go cache.processEvents()

	if interval > 0 {
		go cache.purgeExpired(interval)
	}

	return cache
}

// Capacity returns the maximum capacity of the cache.
func (lru *Cache[K, V]) Capacity() uint64 {
	return lru.capacity
}

// Size returns the current total size of all entries in the cache.
func (lru *Cache[K, V]) Size() uint64 {
	lru.lock.RLock()
	s := lru.size
	lru.lock.RUnlock()
	return s
}

// EntryCount returns the number of entries currently stored in the cache.
func (lru *Cache[K, V]) EntryCount() uint64 {
	lru.lock.RLock()
	l := len(lru.cache)
	lru.lock.RUnlock()
	return uint64(l)
}

// Close gracefully shuts down the cache, stopping background operations.
func (lru *Cache[K, V]) Close() {
	lru.close.Do(func() {
		if lru.purgeInterval > 0 {
			// We need this to block so we don't close the channel until the purge is done.
			lru.done <- true
		}
		close(lru.events)
	})
}

// Set adds a key-value pair to the cache with a default size of 1 and no expiry.
// If the key already exists, the old value is replaced.
func (lru *Cache[K, V]) Set(k K, v V) error {
	return lru.SetWithSizeAndExpiry(k, v, 1, time.Time{})
}

// SetWithSize adds a key-value pair to the cache with a specified size and no expiry.
func (lru *Cache[K, V]) SetWithSize(k K, v V, size uint64) error {
	return lru.SetWithSizeAndExpiry(k, v, size, time.Time{})
}

// SetWithExpiry adds a key-value pair to the cache with no size specified and an expiry time.
func (lru *Cache[K, V]) SetWithExpiry(k K, v V, expires time.Time) error {
	return lru.SetWithSizeAndExpiry(k, v, 1, expires)
}

// SetWithSizeAndExpiry adds a key-value pair to the cache with a specified size and expiry time.
// If the size exceeds the cache's capacity or the expiry time is in the past, an error is returned.
func (lru *Cache[K, V]) SetWithSizeAndExpiry(k K, v V, size uint64, expires time.Time) error {

	if size == 0 {
		return fmt.Errorf("%w: item size = %d", ErrItemTooSmall, size)
	}

	if size > lru.capacity {
		return fmt.Errorf("%w: item size = %d. cache capacity = %d", ErrItemTooBig, size, lru.capacity)
	}

	if !expires.IsZero() && expires.Before(time.Now()) {
		return fmt.Errorf("%w. expires is set to %s, but the current time is %s", ErrPastExpiry, expires.Format(DateTime), time.Now().Format(DateTime))
	}

	n := &node[K, V]{
		key:     k,
		value:   v,
		size:    size,
		expires: expires,
	}

	lru.lock.Lock()

	// Remove the old entry if it exists.
	if existing, found := lru.cache[k]; found {
		lru.deleteNode(existing)
	}

	spaceAvailable := lru.capacity - lru.size
	if spaceAvailable < size {
		if PurgeExpiredEventsWhenCacheIsFull {
			lru.events <- event[K, V]{a: EventActionRemoveExpired}
		}

		wg := &sync.WaitGroup{}
		wg.Add(1)
		lru.events <- event[K, V]{a: EventActionMakeSpaceFor, n: n, finished: wg}
		wg.Wait()
	}

	// Add the new node to the cache and update the size.
	lru.cache[k] = n
	lru.size = lru.size + n.size

	lru.lock.Unlock()

	// Move the new node to the front of the list.
	lru.events <- event[K, V]{a: EventActionAddToFront, n: n}
	return nil
}

// Get retrieves the value associated with the given key from the cache.
// If the key does not exist or has expired, the zero value for the value type is returned.
func (lru *Cache[K, V]) Get(k K) (V, bool) {
	lru.lock.RLock()
	n, found := lru.cache[k]
	lru.lock.RUnlock()

	if !found || n == nil {
		return lru.emptyV, false
	}

	// Check if the node has expired.
	if !n.expires.IsZero() && n.expires.Before(time.Now()) {
		// We'll opt to not remove the expired node here in returning for a quicker return.
		// We say found is false as we treat expired nodes as if they don't exist from the caller's perspective.
		return lru.emptyV, false
	}

	// Move the accessed node to the front of the list.
	lru.events <- event[K, V]{a: EventActionAddToFront, n: n}
	return n.value, true
}

// Delete removes the entry associated with the given key from the cache if it exists.
func (lru *Cache[K, V]) Delete(k K) {
	lru.lock.Lock()
	n, found := lru.cache[k]
	if found {
		lru.deleteNode(n)
	}
	lru.lock.Unlock()
}
