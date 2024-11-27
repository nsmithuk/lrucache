# Least Recently Used Cache, written in Go

_A cache that holds onto the things you use often, and eventually forgets about the things you rarely use._

A thread-safe in-memory cache implementing a Least Recently Used (LRU) eviction policy. Designed
to manage resource constraints efficiently, it keeps frequently accessed data in memory and 
automatically handles the expiration or eviction of older entries.

## Key Goals

- **Capacity Planning**: To provide an in-memory cache that consumes a deterministic maximum amount of memory.
- **Relevance Retention**: To prioritise and retain items that are most relevant, as defined by a **Least Recently Used (LRU)** policy. Items accessed frequently are more likely to remain in the cache, while rarely accessed items are evicted.
- **Performance**: Achieving **O(1)** performance for `Get`, `Set`, and `Delete` operations.
- **Concurrent Access**: Ensuring `Get` operations avoid write locks by using eventual consistency to update the most recently used item, enabling concurrent `Get` requests across multiple threads.

## Features

- **Item Expiry**: Each item can have an associated expiry date. Once expired, the item will no longer be returned from the cache and will eventually be evicted.
- **Background Expiry Purge**: Optionally enable a background task to regularly remove expired items, freeing up space for active entries. This task runs at a configurable frequency, though it incurs an **O(n)** cost per run.
- **Configurable Event Buffer**: Control the size of the event buffer used for eventual consistency. Setting the buffer size to zero (the default) disables eventual consistency, providing strong consistency. A positive buffer size enables eventual consistency, allowing asynchronous updates of "least recently used" metadata during `Get()` operations. Note that `Set()` and `Delete()` operations remain strongly consistent.
- **User-Defined Cost Metric**: Define a custom cost size for each item added to the cache. This provides flexibility in managing cache capacity and prioritising items based on application-specific criteria. (More details provided below.)

### Capacity and Cost

The cache supports defining a custom in-memory "cost" for each item added. When creating a new cache, you set its total capacity, and each item you add can have a size relative to this capacity. This allows you to manage the cache based on your application's specific requirements.

#### Simple Example

If all items have the same cost, you can set the capacity to reflect the total number of items you want the cache to hold. For example, to allow up to 100 items:

- Set the cache's capacity to `100`.
- Define the size of each item as `1` (the default).

This will result in a cache that holds up to 100 items.

#### More Complex Example

For cases where items have varying sizes, you can use a custom cost metric. For instance, if you want to limit the total size of items in the cache to **100 megabytes**:

- Set the cache's capacity in bytes, e.g., `104,857,600` bytes for 100 MB.
- Specify the size of each item in bytes when adding it to the cache.

This ensures the combined size of all items in the cache never exceeds 100 MB (excluding the cache's overhead).

#### Key Notes

- The size of an item does not affect its **ordering** or **retention prioritisation** in the cache. It only determines how much of the cache's total capacity the item consumes.
- It is the responsibility of the user/caller to define a suitable capacity unit for the cache and determine the size of each item. For example, if you choose bytes as the unit:
    - You must calculate the size of each item in bytes before calling `Set()`.
    - Ensure consistent use of bytes as the unit for that cache instance.

## Getting Started

### 1. Create a Cache
You can create an LRU cache with a specified capacity, buffer size and expiry interval with:
```go
package main

import (
  "github.com/yourusername/lrucache"
  "time"
)

func main() {
  // A new LRU Cache with:
  // - Key type: int
  // - Value type: string
  // - Capacity: 100
  // - Buffer: 10
  // - Expiry Interval: 60 seconds
  cache := lrucache.NewCacheWithBufferAndInterval[int, string](100, 10, time.Second * 60)
  defer cache.Close()
}
```

> [!IMPORTANT]
> Call `cache.Close()` when you are finished with the cache to ensure that all associated goroutines are properly terminated.


#### Parameter Details

- **Key Type** (`int` in the example):  
  This can be any type that satisfies Go's `comparable` constraint. It represents the key used in the cache's key-value lookup.

- **Value Type** (`string` in the example):  
  This can be of any type. It represents the value that is stored in and returned from the cache.

- **Capacity** (`uint64`):  
  Specifies the maximum sum of the sizes of all items in the cache (e.g., `100` in the example). For more details, refer to the **Capacity and Cost** section above.

- **Buffer** (`uint16`):  
  Specifies the maximum number of items that can be queued for eventual consistency (`10` in the example).
  - A higher value supports more `Get()` requests in quick succession without blocking but may cause `Set()` or `Delete()` requests to block until the buffer is processed.
  - For strong consistency (no eventual consistency), set this to `0`.
  - Choosing an optimal buffer size is highly dependent on your use case and might require experimentation.

- **Expiry Interval** (`time.Duration`):  
  Specifies the frequency at which expired items are purged from the cache (`60 seconds` in the example).
  - A higher frequency frees up space more quickly by removing expired items, potentially reducing the need to evict tail nodes (which may not have expired).
  - Purging expired items requires a full lock and has an **O(n)** time complexity. Choose a value that balances the benefits of freeing space with the cost of the operation.
  - Set this to `0` to disable periodic purging. If no items in your cache have expiry dates, you should always disable this.

A few helper functions exist for creating a new cache using default values for the _Buffer_ and _Expiry Interval_.
```go
  // Creates a cache with:
  // - A default buffer size of 0 (strong consistency).
  // - An expiry interval of 60 seconds.
  cache := lrucache.NewCacheWithInterval[int, string](100, time.Second * 60)

  // Creates a cache with:
  // - A buffer size of 10 (eventual consistency enabled).
  // - A default expiry interval of 0 (disabled).
  cache := lrucache.NewCacheWithBuffer[int, string](100, 10)

  // Creates a cache with:
  // - A default buffer size of 0 (strong consistency).
  // - A default expiry interval of 0 (disabled).
  cache := lrucache.NewCache[int, string](100)
```

---
### 2. Add items to the cache

You can add items in to the cache using the `Set` method or its variants, depending on whether you want to specify additional details like the item's size or expiry time.

#### Basic Insertion
Use `Set` to add a key-value pair with a default size of 1 and no expiry:
```go
cache.Set(1, "value1")
```

#### Specifying Size
If your items have varying sizes and no expiry, use `SetWithSize` to add a key-value pair and specify its size:
```go
cache.SetWithSize(2, "value2", 5) // Adds the item with a size of 5
```

#### Setting an Expiry Time
Use `SetWithExpiry` to add a key-value pair, with a default size of 1, that expires after a specified duration:
```go
expiry := time.Now().Add(10 * time.Minute) // Item expires in 10 minutes
cache.SetWithExpiry(3, "value3", expiry)
```

#### Combining Size and Expiry
You can use SetWithSizeAndExpiry to add a key-value pair with both size and expiry details:
```go
expiry := time.Now().Add(5 * time.Minute) // Item expires in 5 minutes
cache.SetWithSizeAndExpiry(4, "value4", 3, expiry) // Adds item with size 3
```

#### Errors
The Set methods return an error if:
- The item's size is `< 1`.
- The item's size exceeds the cache's total capacity.
- The specified expiry time is in the past.

---
### 3. Get items from the cache

You can retrieve items from the cache using the `Get` method. This method returns the value associated with the key and a boolean indicating whether the item was found.
```go
value, found := cache.Get(1)
if found {
	fmt.Println("Found:", value)
} else {
	fmt.Println("Key not found or has expired")
}
```
- If the key exists in the cache and has not expired, the value is returned along with true.
- If the key does not exist or the item has expired, the zero value for the value's type is returned along with false.

#### Notes
- You cannot distinguish between an item that does not exist and an item that has expired. We always treat an expired item as if it does not exist from the caller's perspective.
- Items accessed with Get are considered "used" and will be moved to the front of the LRU list. This ensures frequently accessed items remain in the cache.
- If the cache is configured with eventual consistency (a non-zero buffer size), the "most recently used" status may be updated asynchronously.

---
### 4. Delete items from the cache

You can remove items from the cache using the `Delete` method. This method ensures that the specified key and its associated value are removed from the cache, if they exist.

To delete an item by its key:
```go
cache.Delete(1)
```
- If the key exists in the cache, the associated item is removed.
- If the key does not exist, the operation does nothing.

#### Notes
- Deleting an item does not return an error or confirmation, as it is safe to call Delete on non-existent keys.
- Once an item is deleted, it will no longer be accessible through the Get method, even if it was not expired.
- The deletion operation is strongly consistent, regardless of the cache's configuration for eventual consistency.

## License

This project is released under the MIT license, a copy of which can be found in [LICENSE](LICENSE).
