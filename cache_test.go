package lrucache

import (
	"fmt"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"math/rand"
	"slices"
	"sync"
	"testing"
	"time"
)

func getListLength(head *node[int, string]) int {
	count := 0
	for i := 0; i < 1000000; i++ {
		if head.next == nil {
			break
		}
		count++
		head = head.next
	}
	// -1 to remove the tail node from the count.
	return count - 1
}

// GenerateRandomString generates a random string of the specified length.
func generateRandomString(length int) string {
	const charset = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	seededRand := rand.New(rand.NewSource(time.Now().UnixNano()))
	b := make([]byte, length)
	for i := range b {
		b[i] = charset[seededRand.Intn(len(charset))]
	}
	return string(b)
}

// GenerateRandomStringsWithLengths generates random strings of varying lengths.
func generateRandomStringsWithLengths(count, minLength, maxLength int) []string {
	seededRand := rand.New(rand.NewSource(time.Now().UnixNano()))
	strings := make([]string, count)
	for i := 0; i < count; i++ {
		length := seededRand.Intn(maxLength-minLength+1) + minLength
		strings[i] = generateRandomString(length)
	}
	return strings
}

// --------------------------------

func TestCache_SetAndGetBasic(t *testing.T) {
	//Validates basic Set and Get functionality by ensuring that a value can be added to the cache,
	//retrieved successfully, and that requesting a non-existent key returns an empty result.

	// Initialize a new cache with a capacity of 10
	cache := NewCache[int, string](10)
	defer cache.Close()

	// Add a key-value pair to the cache
	err := cache.Set(1, "value1")
	assert.NoError(t, err)

	// Retrieve the value from the cache
	value, found := cache.Get(1)
	assert.Equal(t, "value1", value)
	assert.True(t, found)

	// Test getting a non-existing key
	nonExistentValue, nonExistentFound := cache.Get(2)
	assert.Empty(t, nonExistentValue)
	assert.False(t, nonExistentFound)
}

func TestCache_EvictsOldEntries(t *testing.T) {
	// Tests the cache's eviction policy by filling the cache beyond its capacity and verifying that the oldest
	//entries are evicted while the most recent entries are retained.

	// Initialize a new cache with a capacity of 10
	cache := NewCache[int, string](10)
	defer cache.Close()

	for i := 1; i <= 100; i++ {
		cache.Set(i, fmt.Sprintf("value-%d", i))
	}
	for i := 1; i <= 90; i++ {
		v, _ := cache.Get(i)
		require.Empty(t, v)
	}
	for i := 91; i <= 100; i++ {
		v, _ := cache.Get(i)
		require.Equal(t, fmt.Sprintf("value-%d", i), v)
	}

	assert.Equal(t, uint64(10), cache.EntryCount())
	assert.Equal(t, uint64(10), cache.Size())
	assert.Equal(t, uint64(10), cache.Capacity())
}

func TestCache_ConcurrentAccess(t *testing.T) {
	// Ensures thread safety by concurrently setting values in the cache from multiple goroutines and verifying
	//the correctness of cache contents afterward.

	// Initialize a new cache with a capacity of 10
	cache := NewCache[int, string](10)
	defer cache.Close()

	wg := &sync.WaitGroup{}
	for i := 1; i <= 1000; i++ {
		wg.Add(1)
		go func() {
			cache.Set(i, fmt.Sprintf("value-%d", i))
			wg.Done()
		}()
	}

	wg.Wait()

	emptyAnswers := 0
	nonEmptyAnswers := 0
	for i := 1; i <= 1000; i++ {
		v, _ := cache.Get(i)
		if len(v) == 0 {
			emptyAnswers++
		} else {
			nonEmptyAnswers++
		}
	}

	assert.Equal(t, 990, emptyAnswers)
	assert.Equal(t, 10, nonEmptyAnswers)
	assert.Len(t, cache.cache, 10)
	assert.Equal(t, 10, getListLength(cache.head))
}

func TestCache_MRUOrderingAfterGet(t *testing.T) {
	// Confirms that accessing an entry updates its position in the Most Recently Used (MRU) order, ensuring
	//it is moved to the front of the linked list.

	// Initialize a new cache with a capacity of 10
	cache := NewCache[int, string](1000000)
	defer cache.Close()

	for i := 1; i <= 1000000; i++ {
		cache.Set(i, fmt.Sprintf("value-%d", i))
	}

	tests := []int{12, 123, 1234, 12345, 123456}

	for _, i := range tests {
		v, _ := cache.Get(i)
		assert.Equal(t, fmt.Sprintf("value-%d", i), v)
	}

	time.Sleep(1 * time.Second)

	// We expect this list (backwards) to be at the front of the linked list.
	slices.Reverse(tests)

	head := cache.head
	for _, i := range tests {
		head = head.next
		assert.Equal(t, i, head.key)
	}

}

func TestCache_AutoPurgeExpiredEntries(t *testing.T) {
	// Verifies that expired entries are automatically purged when the cache reaches capacity and new
	//entries are added.

	PurgeExpiredEventsWhenCacheIsFull = true

	cache := NewCache[int, string](3)
	defer cache.Close()

	for i := 1; i <= 3; i++ {
		cache.SetWithExpiry(i, fmt.Sprintf("value-%d", i), time.Now().Add(1*time.Second))
	}

	time.Sleep(2 * time.Second)

	// This should trigger a purge of expired nodes, thus at the end, should be the only one left.
	cache.Set(4, "value-4")
	v, _ := cache.Get(4)
	assert.Equal(t, "value-4", v)
	assert.Len(t, cache.cache, 1)
	assert.Equal(t, 1, getListLength(cache.head))

	PurgeExpiredEventsWhenCacheIsFull = DefaultPurgeExpiredEventsWhenCacheIsFull
}

func TestCache_DeleteEntry(t *testing.T) {
	//Tests the deletion of specific cache entries, ensuring the entry is removed, the linked list is
	//updated correctly, and the cache size reflects the change.

	cache := NewCache[int, string](3)
	defer cache.Close()

	for i := 1; i <= 3; i++ {
		cache.Set(i, fmt.Sprintf("value-%d", i))
	}

	v, _ := cache.Get(3)
	assert.Equal(t, "value-3", v)
	cache.Delete(3)

	v, _ = cache.Get(3)
	assert.Empty(t, v)

	assert.Equal(t, 2, cache.head.next.key)

	assert.Len(t, cache.cache, 2)
	assert.Equal(t, 2, getListLength(cache.head))
}

func TestCache_GetExpiredEntry(t *testing.T) {
	//Checks the behaviour of accessing an expired entry, ensuring the cache does not return values for expired
	//keys and correctly removes them from the data structures.

	cache := NewCache[int, string](3)
	defer cache.Close()

	i := 1
	cache.SetWithExpiry(i, fmt.Sprintf("value-%d", i), time.Now().Add(1*time.Second))

	v, _ := cache.Get(i)
	assert.Equal(t, fmt.Sprintf("value-%d", i), v)
	assert.Len(t, cache.cache, 1)
	assert.Equal(t, 1, getListLength(cache.head))

	time.Sleep(2 * time.Second)

	v, found := cache.Get(i)
	assert.Empty(t, v)
	assert.False(t, found)
}

func TestCache_SetWithBuffer(t *testing.T) {
	//Tests the functionality of setting values with an internal buffer size and ensures MRU ordering
	//is maintained for recently accessed entries.

	// Initialize a new cache with a capacity of 10
	cache := NewCacheWithBuffer[int, string](1000000, 3)
	defer cache.Close()

	for i := 1; i <= 1000000; i++ {
		cache.Set(i, fmt.Sprintf("value-%d", i))
	}

	tests := []int{12, 123, 1234, 12345, 123456}

	for _, i := range tests {
		v, _ := cache.Get(i)
		assert.Equal(t, fmt.Sprintf("value-%d", i), v)
	}

	time.Sleep(1 * time.Second)

	// We expect this list (backwards) to be at the front of the linked list.
	slices.Reverse(tests)

	head := cache.head
	for _, i := range tests {
		head = head.next
		assert.Equal(t, i, head.key)
	}

}

func TestCache_CapacityEnforcedBySize(t *testing.T) {
	//Ensures the cache enforces its capacity not by entry count but by the cumulative size of the
	//stored values, respecting the provided size limit.

	// The capacity here will represent string length
	cache := NewCache[int, string](100)
	defer cache.Close()

	tests := generateRandomStringsWithLengths(50, 10, 20)

	totalSizeSeen := 0
	for i, s := range tests {
		l := len(s)
		totalSizeSeen = totalSizeSeen + l
		cache.SetWithSize(i, s, uint64(l))
	}

	// There should be at least 5 nodes.
	assert.GreaterOrEqual(t, cache.EntryCount(), uint64(5))

	// At most there should be 10 nodes.
	assert.LessOrEqual(t, cache.EntryCount(), uint64(10))
}

func TestCache_ErrorHandling(t *testing.T) {
	// Validates proper error handling for operations that exceed capacity or set entries with invalid
	//parameters such as past expiry times.

	cache := NewCache[int, string](5)
	defer cache.Close()

	// When the size is the same size as the cache, we're okay...
	err := cache.SetWithSize(1, "value1", 5)
	assert.NoError(t, err)

	// But we get an error if it's bigger
	err = cache.SetWithSize(1, "value1", 6)
	assert.Error(t, err)
	assert.ErrorIs(t, err, ErrItemTooBig)

	//---

	// THe size cannot be zero.
	err = cache.SetWithSize(1, "value1", 0)
	assert.Error(t, err)
	assert.ErrorIs(t, err, ErrItemTooSmall)

	//---

	// This is a valid size, but has an expiry set in the past
	err = cache.SetWithSizeAndExpiry(1, "value1", 1, time.Now().Add(-1*time.Second))
	assert.Error(t, err)
	assert.ErrorIs(t, err, ErrPastExpiry)
}

func TestCache_OverwriteEntry(t *testing.T) {
	//Tests overwriting an existing entry in the cache with the same key but a different value, ensuring
	//the cache reflects the updated value and maintains correct size.

	cache := NewCache[int, string](5)
	defer cache.Close()

	err := cache.Set(1, "value1a")
	assert.NoError(t, err)
	v, _ := cache.Get(1)
	assert.Equal(t, "value1a", v)
	assert.Len(t, cache.cache, 1)
	assert.Equal(t, 1, getListLength(cache.head))
	assert.Equal(t, uint64(1), cache.Size())

	// Same key, different value,

	err = cache.Set(1, "value1b")
	assert.NoError(t, err)
	v, _ = cache.Get(1)
	assert.Equal(t, "value1b", v)
	assert.Len(t, cache.cache, 1)
	assert.Equal(t, 1, getListLength(cache.head))
	assert.Equal(t, uint64(1), cache.Size())

}

func TestCache_DeleteAllEntries(t *testing.T) {
	//Simulates multiple full cache deletions and reinsertions, ensuring the cache correctly handles these
	//operations and resets its internal state after all entries are deleted.

	cache := NewCache[int, string](10)
	defer cache.Close()

	for i := 1; i <= 1000000; i++ {
		cache.Set(i, fmt.Sprintf("value-%d", i))
	}
	for i := 1; i <= 1000000; i++ {
		cache.Delete(i)
	}

	for i := 1; i <= 1000000; i++ {
		cache.Set(i, fmt.Sprintf("value-%d", i))
	}
	for i := 1; i <= 1000000; i++ {
		cache.Delete(i)
	}

	for i := 1; i <= 1000000; i++ {
		cache.Set(i, fmt.Sprintf("value-%d", i))
	}
	for i := 1; i <= 1000000; i++ {
		cache.Delete(i)
	}

	assert.Equal(t, uint64(0), cache.EntryCount())
	assert.Equal(t, uint64(0), cache.Size())

	// When there are no nodes left, the head and tail should point at each other.
	assert.Equal(t, cache.tail, cache.head.next)
	assert.Equal(t, cache.head, cache.tail.previous)
}

func TestCache_TimerBasedPurge(t *testing.T) {
	//Tests the periodic purge functionality to ensure that entries with expired timestamps are removed
	//correctly during scheduled intervals.

	cache := NewCacheWithInterval[int, string](10, time.Millisecond*50)
	defer cache.Close()

	for i := 1; i <= 3; i++ {
		cache.SetWithExpiry(i, fmt.Sprintf("value-%d", i), time.Now().Add(150*time.Millisecond))
	}

	time.Sleep(time.Millisecond * 50)

	assert.Equal(t, uint64(3), cache.Size())
	for i := 1; i <= 3; i++ {
		v, _ := cache.Get(i)
		assert.Equal(t, fmt.Sprintf("value-%d", i), v)
	}

	time.Sleep(time.Millisecond * 50)

	assert.Equal(t, uint64(3), cache.Size())
	for i := 1; i <= 3; i++ {
		v, _ := cache.Get(i)
		assert.Equal(t, fmt.Sprintf("value-%d", i), v)
	}

	time.Sleep(time.Millisecond * 100)

	// Everything should now be empty.
	assert.Equal(t, uint64(0), cache.Size())
	assert.Equal(t, uint64(0), cache.EntryCount())
}

func TestCache_CorrectSizeCalculationOnUpdate(t *testing.T) {

	cache := NewCache[int, string](1)
	defer cache.Close()

	err := cache.Set(1, "value1")
	assert.NoError(t, err)

	assert.Equal(t, uint64(1), cache.EntryCount())
	assert.Equal(t, uint64(1), cache.Size())

	//---

	err = cache.Set(1, "value1-updated")
	assert.NoError(t, err)

	assert.Equal(t, uint64(1), cache.EntryCount())
	assert.Equal(t, uint64(1), cache.Size())

	cache.Get(1)
	cache.Get(1)
	cache.Get(1)
	cache.Set(2, "value2")

	_, found := cache.Get(1)
	assert.False(t, found)

	_, found = cache.Get(2)
	assert.True(t, found)

	assert.Equal(t, uint64(1), cache.EntryCount())
	assert.Equal(t, uint64(1), cache.Size())
}

func TestCache_Something(t *testing.T) {

	cache := NewCache[int, string](1)
	defer cache.Close()

}
