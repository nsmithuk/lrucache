package lrucache

import(
	"container/list"
	"sync"
	"errors"
)

type Cache struct {
	order *list.List

	items map[string]*Item

	lock sync.Mutex

	maxSize uint64
	currentSize uint64
}

type Item struct {
	size uint64
	value interface{}
	listElement *list.Element
}

func New(sizeInBytes uint64) *Cache {

	return &Cache{
		order: list.New(),
		maxSize: sizeInBytes,
		currentSize: 0,
		items: make(map[string]*Item),
	}

}


func (c *Cache) Set(key string, value interface{}, size uint64) error {
	c.lock.Lock()
	defer c.lock.Unlock()

	/*
		- If the item already exists in the map
			- Delete it and continue added the new replacement
		- Else check if the item size is <= the total allowed cache size
			- If not, reject
		- Else check if there's enough room to add the new item into the cache
			- If not, remove one item at a time off the back of the queue until there is enough room
		- Add the new item
	 */

	// If the key already has a value, remove it.
	if existingItem, ok := c.items[key]; ok {
		c.removeItem(existingItem)
	}

	//---

	// Protect against adding an item bigger than the total allowed size.
	if size > c.maxSize {
		return errors.New("value is larger than max cache size")
	}

	//---

	// If the new value cannot currently fit in the cache, we prune...
	for (c.maxSize - c.currentSize) < size {

		// Return the item on the back of the queue
		lastElement := c.order.Back()

		// Find the corresponding Item in the map
		lastItem := c.items[lastElement.Value.(string)]

		c.removeItem(lastItem)
	}

	//---

	// Add the new key to the front of the list
	listElement := c.order.PushFront(key)

	// Create the item
	item := &Item{
		value: value,
		size: size,
		listElement: listElement,
	}

	// Store the item in the map
	c.items[key] = item

	c.currentSize += size

	return nil
}

func (c *Cache) Get(key string) (value interface{}, ok bool) {
	c.lock.Lock()
	defer c.lock.Unlock()

	item, ok := c.items[key]

	if !ok {
		return nil, false
	}

	// Return the item to teh front of the queue
	c.order.MoveToFront(item.listElement)

	return item.value, true
}

func (c *Cache) Delete(key string) {
	c.lock.Lock()
	defer c.lock.Unlock()

	if existingItem, ok := c.items[key]; ok {
		c.removeItem(existingItem)
	}
}

//----------------------------------------------------

func (c *Cache) removeItem(item *Item) {

	key := item.listElement.Value.(string)

	// Decrease the used size by the item size
	c.currentSize -= item.size

	// Remove the map and list element/item.
	delete(c.items, key)
	c.order.Remove(item.listElement)
}