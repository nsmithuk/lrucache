# Least Recently Used Cache, written in Go

A simple thread-safe LRU cache, written in go. The cache's cap is based on the
number of bytes stored within it, as opposed to the number of items.

```sh
go get -u github.com/NSmithUK/lru-cache-go
```

Create a new cache:
```go
cache := lrucache.New( <max size in bytes> )
```

Adding an item:
```go
err := cache.Set(key, value, <value size in bytes>)
```

Getting an item:
```go
value, found := cache.Get(key)
```

Removing an item:
```go
cache.Delete(key)
```
