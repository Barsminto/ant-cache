package cache

import (
	"ant-cache/auth"
	"bytes"
	"container/heap"
	"fmt"
	"sync"
	"time"
)

type CacheItem struct {
	Value      interface{}
	Expiration int64
	index      int    // for heap operations
	key        string // for deletion operations
	Type       string // string, array, object
}

// Memory pools for reducing GC pressure
var (
	// Buffer pool for string operations
	bufferPool = sync.Pool{
		New: func() interface{} {
			return bytes.NewBuffer(make([]byte, 0, 1024))
		},
	}

	// CacheItem pool for reusing cache items
	itemPool = sync.Pool{
		New: func() interface{} {
			return &CacheItem{}
		},
	}

	// String slice pool for batch operations
	stringSlicePool = sync.Pool{
		New: func() interface{} {
			return make([]string, 0, 100)
		},
	}
)

// BatchOperation represents a batch of cache operations
type BatchOperation struct {
	Type  string // SET, GET, DEL
	Key   string
	Value interface{}
	TTL   time.Duration
}

// BatchResult represents the result of a batch operation
type BatchResult struct {
	Success bool
	Value   interface{}
	Error   string
}

type Cache struct {
	items map[string]*CacheItem
	mu    sync.RWMutex
	// Min heap for expiration times, used to quickly find the earliest expiring items
	expirationHeap *ExpirationHeap
	// Persistence manager for data persistence
	persistence *PersistenceManager
	// Authentication manager
	authManager *auth.AuthManager
	// Batch processing channel
	batchChan chan []BatchOperation
	// Batch processing workers
	batchWorkers int
	// Compression configuration
	compressionConfig CompressionConfig
}

// ExpirationHeap implements min heap for managing expiration times
type ExpirationHeap []*CacheItem

func (h ExpirationHeap) Len() int { return len(h) }

func (h ExpirationHeap) Less(i, j int) bool {
	return h[i].Expiration < h[j].Expiration
}

func (h ExpirationHeap) Swap(i, j int) {
	h[i], h[j] = h[j], h[i]
	h[i].index = i
	h[j].index = j
}

func (h *ExpirationHeap) Push(x interface{}) {
	n := len(*h)
	item := x.(*CacheItem)
	item.index = n
	*h = append(*h, item)
}

func (h *ExpirationHeap) Pop() interface{} {
	old := *h
	n := len(old)
	item := old[n-1]
	old[n-1] = nil  // avoid memory leak
	item.index = -1 // mark as deleted
	*h = old[0 : n-1]
	return item
}

// Remove removes an item from the heap
func (h *ExpirationHeap) Remove(item *CacheItem) {
	if item.index >= 0 && item.index < len(*h) {
		heap.Remove(h, item.index)
	}
}

func New() *Cache {
	h := &ExpirationHeap{}
	heap.Init(h)
	return &Cache{
		items:             make(map[string]*CacheItem),
		expirationHeap:    h,
		compressionConfig: DefaultCompressionConfig(),
	}
}

// NewWithPersistence create cache with persistence
func NewWithPersistence(atdPath, aclPath string, atdInterval, aclInterval time.Duration) *Cache {
	cache := New()
	if atdPath != "" && aclPath != "" {
		cache.persistence = NewPersistenceManager(cache, atdPath, aclPath, atdInterval, aclInterval)
		// Load data when starting
		if err := cache.persistence.LoadAtd(); err != nil {
			// Loading data does not affect startup, just print the error
			fmt.Printf("Failed to load ATD: %v\n", err)
		}
		// Load command log
		if err := cache.persistence.LoadAcl(); err != nil {
			// Loading command log does not affect startup, just print the error
			fmt.Printf("Failed to load ACL: %v\n", err)
		}
		// Start persistence manager
		cache.persistence.Start()
	}
	return cache
}

// NewWithPersistenceAndAuth create cache with persistence and authentication
func NewWithPersistenceAndAuth(atdPath, aclPath string, atdInterval, aclInterval time.Duration, authManager *auth.AuthManager) *Cache {
	cache := NewWithPersistence(atdPath, aclPath, atdInterval, aclInterval)
	cache.authManager = authManager
	return cache
}

// Close the cache and save the data
func (c *Cache) Close() {
	if c.persistence != nil {
		c.persistence.Stop()
	}
}

// SetCompressionConfig sets the compression configuration
func (c *Cache) SetCompressionConfig(config CompressionConfig) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.compressionConfig = config
}

func (c *Cache) Set(key string, value interface{}, ttl time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()

	// Determine data type
	var dataType string
	switch value.(type) {
	case []string:
		dataType = "array"
	case map[string]string:
		dataType = "object"
	default:
		dataType = "string"
	}

	// Try to compress the value if compression is enabled
	compressedValue, err := CompressValue(value, dataType, c.compressionConfig)
	if err != nil {
		// If compression fails, use the original value
		fmt.Printf("Compression failed for key %s: %v\n", key, err)
		compressedValue = value
	}

	item := &CacheItem{
		Value: compressedValue,
		key:   key,
		Type:  dataType,
	}

	// Only set expiration time when ttl > 0
	if ttl > 0 {
		item.Expiration = time.Now().Add(ttl).UnixNano()
		// Add to expiration heap
		heap.Push(c.expirationHeap, item)
	}

	c.items[key] = item

	// Removed stats tracking

	// Log the set command to the command log
	if c.persistence != nil {
		c.persistence.LogCommand("SET", key, value, ttl)
	}
}

// SetNX sets a key only if it doesn't exist (atomic operation)
func (c *Cache) SetNX(key string, value interface{}, ttl time.Duration) bool {
	c.mu.Lock()
	defer c.mu.Unlock()

	// Check if key already exists and is not expired
	if item, exists := c.items[key]; exists {
		// Check if item is expired
		if item.Expiration > 0 && time.Now().UnixNano() > item.Expiration {
			// Item is expired, we can set it
		} else {
			// Key exists and is not expired, return false
			return false
		}
	}

	// Determine data type
	var dataType string
	switch value.(type) {
	case []string:
		dataType = "array"
	case map[string]string:
		dataType = "object"
	default:
		dataType = "string"
	}

	// Try to compress the value if compression is enabled
	compressedValue, err := CompressValue(value, dataType, c.compressionConfig)
	if err != nil {
		// If compression fails, use the original value
		fmt.Printf("Compression failed for key %s: %v\n", key, err)
		compressedValue = value
	}

	item := &CacheItem{
		Value: compressedValue,
		key:   key,
		Type:  dataType,
	}

	// Only set expiration time when ttl > 0
	if ttl > 0 {
		item.Expiration = time.Now().Add(ttl).UnixNano()
		// Add to expiration heap
		heap.Push(c.expirationHeap, item)
	}

	c.items[key] = item

	// Log the set command to the command log
	if c.persistence != nil {
		c.persistence.LogCommand("SETNX", key, value, ttl)
	}

	return true
}

func (c *Cache) Get(key string) (interface{}, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	item, found := c.items[key]
	if !found {
		// stats tracking removed
		return nil, false
	}

	if item.Expiration > 0 && time.Now().UnixNano() > item.Expiration {
		// stats tracking removed
		return nil, false
	}

	// Try to decompress the value if it's compressed
	decompressedValue, _, err := DecompressValue(item.Value)
	if err != nil {
		// If decompression fails, return the original value
		fmt.Printf("Decompression failed for key %s: %v\n", key, err)
		return item.Value, true
	}

	// Removed stats tracking
	return decompressedValue, true
}

// GetMultiple gets multiple keys at once
func (c *Cache) GetMultiple(keys []string) map[string]interface{} {
	c.mu.RLock()
	defer c.mu.RUnlock()

	result := make(map[string]interface{})
	now := time.Now().UnixNano()

	for _, key := range keys {
		if item, found := c.items[key]; found {
			// Check if item is expired
			if item.Expiration > 0 && now > item.Expiration {
				continue // Skip expired items
			}

			// Try to decompress the value if it's compressed
			decompressedValue, _, err := DecompressValue(item.Value)
			if err != nil {
				// If decompression fails, use the original value
				fmt.Printf("Decompression failed for key %s: %v\n", key, err)
				result[key] = item.Value
			} else {
				result[key] = decompressedValue
			}
		}
	}

	return result
}

func (c *Cache) Delete(key string) bool {
	c.mu.Lock()
	defer c.mu.Unlock()

	if item, exists := c.items[key]; exists {
		// If has expiration time, remove from heap
		if item.Expiration > 0 && item.index >= 0 {
			heap.Remove(c.expirationHeap, item.index)
		}
		delete(c.items, key)

		// Removed stats tracking

		// Log the delete command to the command log
		if c.persistence != nil {
			c.persistence.LogCommand("DEL", key, "", 0)
		}
		return true
	}
	return false
}

// DeleteString method removed - use Delete instead

// DeleteArray method removed - use Delete instead

// DeleteObject method removed - use Delete instead

func (c *Cache) Cleanup() {
	c.mu.Lock()
	defer c.mu.Unlock()

	now := time.Now().UnixNano()

	// Only check the top of the heap for expired items, avoid traversing all keys
	for c.expirationHeap.Len() > 0 {
		item := (*c.expirationHeap)[0]
		if item.Expiration > now {
			// Top item not expired yet, stop checking
			break
		}

		heap.Pop(c.expirationHeap)
		delete(c.items, item.key)
	}
}

// GetStats method removed

// GetAuthManager obtain the authentication manager
func (c *Cache) GetAuthManager() *auth.AuthManager {
	return c.authManager
}

// Keys returns all keys matching the pattern (optimized with memory pool)
func (c *Cache) Keys(pattern string) []string {
	c.mu.RLock()
	defer c.mu.RUnlock()

	// Get slice from pool
	keys := stringSlicePool.Get().([]string)
	keys = keys[:0] // Reset length but keep capacity
	defer stringSlicePool.Put(keys)

	now := time.Now().UnixNano()

	for key, item := range c.items {
		// Skip expired items
		if item.Expiration > 0 && item.Expiration < now {
			continue
		}

		// Simple pattern matching (only supports "*" for all keys)
		if pattern == "*" {
			keys = append(keys, key)
		}
		// Could add more pattern matching logic here if needed
	}

	// Return a copy since we're putting the slice back to pool
	result := make([]string, len(keys))
	copy(result, keys)
	return result
}

// GetBuffer gets a buffer from the pool
func GetBuffer() *bytes.Buffer {
	return bufferPool.Get().(*bytes.Buffer)
}

// PutBuffer returns a buffer to the pool
func PutBuffer(buf *bytes.Buffer) {
	buf.Reset()
	bufferPool.Put(buf)
}

// GetCacheItem gets a cache item from the pool
func GetCacheItem() *CacheItem {
	item := itemPool.Get().(*CacheItem)
	// Reset the item
	item.Value = nil
	item.Expiration = 0
	item.index = 0
	item.key = ""
	item.Type = ""
	return item
}

// PutCacheItem returns a cache item to the pool
func PutCacheItem(item *CacheItem) {
	itemPool.Put(item)
}

// BatchExecute executes a batch of operations atomically
func (c *Cache) BatchExecute(operations []BatchOperation) []BatchResult {
	results := make([]BatchResult, len(operations))

	c.mu.Lock()
	defer c.mu.Unlock()

	now := time.Now().UnixNano()

	for i, op := range operations {
		switch op.Type {
		case "SET":
			item := GetCacheItem()
			item.Value = op.Value
			item.key = op.Key
			item.Type = "string"

			if op.TTL > 0 {
				item.Expiration = now + op.TTL.Nanoseconds()
				heap.Push(c.expirationHeap, item)
			}

			// Remove old item if exists
			if oldItem, exists := c.items[op.Key]; exists {
				if oldItem.Expiration > 0 {
					c.expirationHeap.Remove(oldItem)
				}
				PutCacheItem(oldItem)
			}

			c.items[op.Key] = item
			results[i] = BatchResult{Success: true}

		case "GET":
			if item, exists := c.items[op.Key]; exists {
				if item.Expiration == 0 || item.Expiration > now {
					results[i] = BatchResult{Success: true, Value: item.Value}
				} else {
					// Item expired
					delete(c.items, op.Key)
					c.expirationHeap.Remove(item)
					PutCacheItem(item)
					results[i] = BatchResult{Success: false, Error: "key not found"}
				}
			} else {
				results[i] = BatchResult{Success: false, Error: "key not found"}
			}

		case "DEL":
			if item, exists := c.items[op.Key]; exists {
				delete(c.items, op.Key)
				if item.Expiration > 0 {
					c.expirationHeap.Remove(item)
				}
				PutCacheItem(item)
				results[i] = BatchResult{Success: true}
			} else {
				results[i] = BatchResult{Success: false, Error: "key not found"}
			}

		default:
			results[i] = BatchResult{Success: false, Error: "unknown operation"}
		}
	}

	return results
}

// OptimizedSet uses memory pool for better performance
func (c *Cache) OptimizedSet(key string, value interface{}, ttl time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()

	now := time.Now().UnixNano()

	// Get item from pool
	item := GetCacheItem()
	item.Value = value
	item.key = key
	item.Type = "string"

	if ttl > 0 {
		item.Expiration = now + ttl.Nanoseconds()
		heap.Push(c.expirationHeap, item)
	}

	// Remove old item if exists
	if oldItem, exists := c.items[key]; exists {
		if oldItem.Expiration > 0 {
			c.expirationHeap.Remove(oldItem)
		}
		PutCacheItem(oldItem)
	}

	c.items[key] = item
}

// GetAllKeys returns all keys with their metadata
func (c *Cache) GetAllKeys() []map[string]interface{} {
	c.mu.RLock()
	defer c.mu.RUnlock()

	var keys []map[string]interface{}
	now := time.Now().UnixNano()

	// Collect all keys from map
	for key, item := range c.items {
		// Skip expired items
		if item.Expiration > 0 && item.Expiration < now {
			continue
		}

		// Calculate TTL
		var ttl int64 = 0
		if item.Expiration > 0 {
			ttl = (item.Expiration - now) / int64(time.Second)
			if ttl < 0 {
				ttl = 0
			}
		}

		// Calculate size
		size := len(fmt.Sprintf("%v", item.Value))

		keyInfo := map[string]interface{}{
			"key":        key,
			"type":       item.Type,
			"value":      item.Value,
			"ttl":        ttl,
			"expires_at": "",
			"size":       size,
		}

		if item.Expiration > 0 {
			keyInfo["expires_at"] = time.Unix(0, item.Expiration).Format(time.RFC3339)
		}

		keys = append(keys, keyInfo)
	}

	// Keys are returned in map iteration order (no specific sorting)

	return keys
}

// FlushAll removes all keys from the cache
func (c *Cache) FlushAll() int {
	c.mu.Lock()
	defer c.mu.Unlock()

	count := len(c.items)

	// Clear all items
	c.items = make(map[string]*CacheItem)

	// Reset expiration heap
	c.expirationHeap = &ExpirationHeap{}
	heap.Init(c.expirationHeap)

	return count
}
