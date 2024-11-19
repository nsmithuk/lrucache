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

- **Item Expiry**: Each item can have an associated expiry date. Once expired, the item will no longer be returned from the cache and will eventually be removed.
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

#### Complex Example

For cases where items have varying sizes, you can use a custom cost metric. For instance, if you want to limit the total size of items in the cache to **100 megabytes**:

- Set the cache's capacity in bytes, e.g., `104,857,600` bytes for 100 MB.
- Specify the size of each item in bytes when adding it to the cache.

This ensures the combined size of all items in the cache never exceeds 100 MB (excluding the cache's overhead).

#### Key Notes

- The size of an item does not affect its **ordering** or **retention prioritisation** in the cache. It only determines how much of the cache's total capacity the item consumes.
- It is the responsibility of the user/caller to define a suitable capacity unit for the cache and determine the size of each item. For example, if you choose bytes as the unit:
    - You must calculate the size of each item in bytes before calling `Set()`.
    - Ensure consistent use of bytes as the unit for that cache instance.

## License

This project is released under the MIT license, a copy of which can be found in [LICENSE](LICENSE).
