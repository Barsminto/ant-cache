package main

import (
	"ant-cache/cache"
	"fmt"
	"runtime"
	"sync"
	"time"
)

func main() {
	fmt.Println("=== Ant-Cache Performance Test ===")

	// Test different concurrency levels
	concurrencyLevels := []int{1, 10, 100, 1000, 5000, 10000, 50000, 100000}

	for _, concurrency := range concurrencyLevels {
		fmt.Printf("\n--- Concurrency Level: %d ---\n", concurrency)
		testConcurrency(concurrency)
	}

	// Test memory usage
	testMemoryUsage()

	// Test CPU usage
	testCPUUsage()

	// Test cleanup performance
	testCleanupPerformance()
}

func testConcurrency(concurrency int) {
	c := cache.New()

	// For high concurrency tests, warm up cache first
	if concurrency > 1000 {
		fmt.Printf("Warming up cache...\n")
		for i := 0; i < concurrency/10; i++ {
			key := fmt.Sprintf("key_%d", i)
			value := fmt.Sprintf("value_%d", i)
			c.Set(key, value, 0)
		}
	}

	// Test write operations
	start := time.Now()
	var wg sync.WaitGroup

	// Concurrent write test
	for i := 0; i < concurrency; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			key := fmt.Sprintf("key_%d", id)
			value := fmt.Sprintf("value_%d", id)
			c.Set(key, value, 0)
		}(i)
	}
	wg.Wait()
	writeTime := time.Since(start)

	// Test read operations
	start = time.Now()
	for i := 0; i < concurrency; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			key := fmt.Sprintf("key_%d", i%concurrency)
			c.Get(key)
		}(i)
	}
	wg.Wait()
	readTime := time.Since(start)

	// Test mixed operations (reduce mixed operation count for high concurrency)
	mixedOps := concurrency
	if concurrency > 10000 {
		mixedOps = 10000 // Limit mixed operation count for high concurrency
	}

	start = time.Now()
	for i := 0; i < mixedOps; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			key := fmt.Sprintf("key_%d", id)
			if id%2 == 0 {
				c.Set(key, fmt.Sprintf("new_value_%d", id), 0)
			} else {
				c.Get(key)
			}
		}(i)
	}
	wg.Wait()
	mixedTime := time.Since(start)

	// Get cache statistics
	allKeys := c.GetAllKeys()
	itemCount := len(allKeys)

	fmt.Printf("Write operations: %d ops, duration: %v, average: %.2f ops/sec\n",
		concurrency, writeTime, float64(concurrency)/writeTime.Seconds())
	fmt.Printf("Read operations: %d ops, duration: %v, average: %.2f ops/sec\n",
		concurrency, readTime, float64(concurrency)/readTime.Seconds())
	fmt.Printf("Mixed operations: %d ops, duration: %v, average: %.2f ops/sec\n",
		mixedOps, mixedTime, float64(mixedOps)/mixedTime.Seconds())
	fmt.Printf("Cache items: %d\n", itemCount)
}

func testMemoryUsage() {
	fmt.Println("\n--- Memory Usage Test ---")

	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	fmt.Printf("Initial memory: %d MB\n", m.Alloc/1024/1024)

	c := cache.New()

	// Insert large amount of data
	for i := 0; i < 100000; i++ {
		key := fmt.Sprintf("key_%d", i)
		value := fmt.Sprintf("value_%d_%s", i, "This is a very long value for testing memory usage")
		c.Set(key, value, 0)
	}

	runtime.ReadMemStats(&m)
	fmt.Printf("After inserting 100k items: %d MB\n", m.Alloc/1024/1024)

	allKeys := c.GetAllKeys()
	fmt.Printf("Actual cache items: %d\n", len(allKeys))

	// Clean up some data
	for i := 0; i < 50000; i++ {
		key := fmt.Sprintf("key_%d", i)
		c.Delete(key)
	}

	runtime.ReadMemStats(&m)
	fmt.Printf("After deleting 50k items: %d MB\n", m.Alloc/1024/1024)

	allKeys = c.GetAllKeys()
	fmt.Printf("Remaining cache items: %d\n", len(allKeys))
}

func testCPUUsage() {
	fmt.Println("\n--- CPU Usage Test ---")

	c := cache.New()

	// Warm up
	for i := 0; i < 10000; i++ {
		key := fmt.Sprintf("key_%d", i)
		c.Set(key, fmt.Sprintf("value_%d", i), 0)
	}

	// Continuous stress test
	start := time.Now()
	operations := 0

	for time.Since(start) < 5*time.Second {
		for i := 0; i < 1000; i++ {
			key := fmt.Sprintf("key_%d", i%10000)
			if i%3 == 0 {
				c.Set(key, fmt.Sprintf("new_value_%d", i), 0)
			} else {
				c.Get(key)
			}
			operations++
		}
	}

	duration := time.Since(start)
	fmt.Printf("Operations completed in 5 seconds: %d\n", operations)
	fmt.Printf("Average operation speed: %.2f ops/sec\n", float64(operations)/duration.Seconds())

	allKeys := c.GetAllKeys()
	fmt.Printf("Final cache items: %d\n", len(allKeys))
}

func testCleanupPerformance() {
	fmt.Println("\n--- Cleanup Performance Test ---")

	// Test different numbers of expired keys
	testCases := []int{1000, 10000, 50000}

	for _, keyCount := range testCases {
		fmt.Printf("\n--- Testing %d expired keys ---\n", keyCount)
		testCleanupPerformanceForCount(keyCount)
	}
}

func testCleanupPerformanceForCount(keyCount int) {
	c := cache.New()

	// Add large number of keys with expiration time
	fmt.Printf("Adding %d keys with expiration time...\n", keyCount)
	start := time.Now()

	for i := 0; i < keyCount; i++ {
		key := fmt.Sprintf("key_%d", i)
		// Set different expiration times to simulate real scenarios
		ttl := time.Duration(i%60+1) * time.Second // 1-60 seconds TTL
		c.Set(key, fmt.Sprintf("value_%d", i), ttl)
	}

	addTime := time.Since(start)
	fmt.Printf("Add time: %v\n", addTime)

	// Wait for some keys to expire
	fmt.Println("Waiting 3 seconds for some keys to expire...")
	time.Sleep(3 * time.Second)

	// Test cleanup performance
	fmt.Println("Starting cleanup test...")
	cleanupTimes := make([]time.Duration, 5)

	for i := 0; i < 5; i++ {
		start := time.Now()
		c.Cleanup()
		cleanupTimes[i] = time.Since(start)
	}

	// Calculate average cleanup time
	var totalTime time.Duration
	for _, t := range cleanupTimes {
		totalTime += t
	}
	avgTime := totalTime / time.Duration(len(cleanupTimes))

	fmt.Printf("Average cleanup time: %v\n", avgTime)

	// Get statistics
	allKeys := c.GetAllKeys()
	fmt.Printf("Remaining cache items: %d\n", len(allKeys))

	// Calculate performance metrics
	opsPerSec := float64(keyCount) / avgTime.Seconds()
	fmt.Printf("Cleanup performance: %.2f ops/sec\n", opsPerSec)
}
