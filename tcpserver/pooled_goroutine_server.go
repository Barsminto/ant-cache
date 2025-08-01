package tcpserver

import (
	"bufio"
	"encoding/json"
	"fmt"
	"net"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"ant-cache/cache"
	"ant-cache/utils"
)

// ConnectionTask represents a connection handling task for the goroutine pool
type ConnectionTask struct {
	conn   net.Conn
	server *PooledGoroutineServer
}

// GoroutinePool manages an optimized pool of goroutines for direct cache memory operations
type GoroutinePool struct {
	taskChan    chan *ConnectionTask
	workerCount int
	maxWorkers  int
	minWorkers  int
	wg          sync.WaitGroup
	stopChan    chan struct{}

	// Performance metrics
	activeWorkers  int64
	totalTasks     int64
	completedTasks int64
	rejectedTasks  int64
	avgTaskTime    int64 // nanoseconds

	// Dynamic scaling
	lastScaleTime  time.Time
	scaleInterval  time.Duration
	taskTimeWindow []int64 // Rolling window of task times
	windowSize     int
	windowIndex    int
	mu             sync.RWMutex
}

// NewGoroutinePool creates a new optimized goroutine pool
func NewGoroutinePool(workerCount int) *GoroutinePool {
	minWorkers := max(1, workerCount/4)
	maxWorkers := workerCount * 2

	return &GoroutinePool{
		taskChan:       make(chan *ConnectionTask, maxWorkers),
		workerCount:    workerCount,
		minWorkers:     minWorkers,
		maxWorkers:     maxWorkers,
		stopChan:       make(chan struct{}),
		scaleInterval:  time.Second * 5,    // Scale every 5 seconds
		taskTimeWindow: make([]int64, 100), // 100 sample window
		windowSize:     100,
		lastScaleTime:  time.Now(),
	}
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

// Start starts the goroutine pool
func (gp *GoroutinePool) Start() {
	for i := 0; i < gp.workerCount; i++ {
		gp.wg.Add(1)
		go gp.worker(i)
	}
}

// worker is an optimized goroutine that processes connection tasks with performance monitoring
func (gp *GoroutinePool) worker(id int) {
	defer gp.wg.Done()

	atomic.AddInt64(&gp.activeWorkers, 1)
	defer atomic.AddInt64(&gp.activeWorkers, -1)

	for {
		select {
		case task := <-gp.taskChan:
			if task == nil {
				return
			}

			// Measure task execution time
			start := time.Now()
			gp.handleConnectionTask(task)
			duration := time.Since(start).Nanoseconds()

			// Update metrics
			atomic.AddInt64(&gp.completedTasks, 1)
			gp.updateTaskTime(duration)

		case <-gp.stopChan:
			return
		}
	}
}

// updateTaskTime updates the rolling window of task execution times
func (gp *GoroutinePool) updateTaskTime(duration int64) {
	gp.mu.Lock()
	defer gp.mu.Unlock()

	gp.taskTimeWindow[gp.windowIndex] = duration
	gp.windowIndex = (gp.windowIndex + 1) % gp.windowSize

	// Update average (simple moving average)
	var sum int64
	count := 0
	for _, t := range gp.taskTimeWindow {
		if t > 0 {
			sum += t
			count++
		}
	}
	if count > 0 {
		atomic.StoreInt64(&gp.avgTaskTime, sum/int64(count))
	}
}

// handleConnectionTask handles a connection task with direct cache memory operations
func (gp *GoroutinePool) handleConnectionTask(task *ConnectionTask) {
	defer func() {
		task.conn.Close()
		atomic.AddUint64(&task.server.activeConnections, ^uint64(0)) // Decrement
	}()

	// Set connection options for better performance
	if tcpConn, ok := task.conn.(*net.TCPConn); ok {
		tcpConn.SetNoDelay(true)
		tcpConn.SetKeepAlive(true)
		tcpConn.SetKeepAlivePeriod(30 * time.Second)
		tcpConn.SetReadBuffer(32768)  // 32KB read buffer
		tcpConn.SetWriteBuffer(32768) // 32KB write buffer
	}

	task.conn.SetReadDeadline(time.Now().Add(30 * time.Second))

	gp.handleTextConnectionTask(task)
}

// handleTextConnectionTask handles text protocol connection task
func (gp *GoroutinePool) handleTextConnectionTask(task *ConnectionTask) {
	scanner := bufio.NewScanner(task.conn)
	authenticated := false

	// Use larger buffer for better performance
	buf := make([]byte, 0, 64*1024)
	scanner.Buffer(buf, 1024*1024) // 1MB max token size

	for scanner.Scan() {
		line := scanner.Text()
		if line == "" {
			continue
		}

		atomic.AddUint64(&task.server.totalRequests, 1)

		// Reset read deadline
		task.conn.SetReadDeadline(time.Now().Add(30 * time.Second))

		// Process command directly in this pooled goroutine (direct memory access)
		response := task.server.processCommandDirect(line, &authenticated)

		// Send response
		_, err := task.conn.Write([]byte(response))
		if err != nil {
			fmt.Printf("Failed to write response: %v\n", err)
			return
		}

		atomic.AddUint64(&task.server.totalResponses, 1)
	}

	if err := scanner.Err(); err != nil {
		fmt.Printf("Connection read error: %v\n", err)
	}
}

// SubmitTask submits a connection task to the pool with dynamic scaling
func (gp *GoroutinePool) SubmitTask(task *ConnectionTask) bool {
	atomic.AddInt64(&gp.totalTasks, 1)

	select {
	case gp.taskChan <- task:
		// Check if we need to scale
		gp.checkAndScale()
		return true
	case <-time.After(100 * time.Millisecond):
		atomic.AddInt64(&gp.rejectedTasks, 1)
		return false // Pool is full
	}
}

// checkAndScale checks if the pool needs to be scaled up or down
func (gp *GoroutinePool) checkAndScale() {
	now := time.Now()
	if now.Sub(gp.lastScaleTime) < gp.scaleInterval {
		return // Too soon to scale
	}

	gp.mu.Lock()
	defer gp.mu.Unlock()

	if now.Sub(gp.lastScaleTime) < gp.scaleInterval {
		return // Double check after acquiring lock
	}

	activeWorkers := int(atomic.LoadInt64(&gp.activeWorkers))
	queueLength := len(gp.taskChan)
	avgTaskTime := atomic.LoadInt64(&gp.avgTaskTime)

	// Scale up conditions: high queue length or all workers busy
	if (queueLength > gp.workerCount/2 || activeWorkers >= gp.workerCount*9/10) &&
		gp.workerCount < gp.maxWorkers {
		gp.scaleUp()
	}

	// Scale down conditions: low queue length and low average task time
	if queueLength == 0 && avgTaskTime < 1000000 && // < 1ms average
		gp.workerCount > gp.minWorkers {
		gp.scaleDown()
	}

	gp.lastScaleTime = now
}

// scaleUp adds more workers to the pool
func (gp *GoroutinePool) scaleUp() {
	newWorkers := min(gp.workerCount/4, gp.maxWorkers-gp.workerCount)
	if newWorkers <= 0 {
		return
	}

	for i := 0; i < newWorkers; i++ {
		gp.wg.Add(1)
		go gp.worker(gp.workerCount + i)
	}

	gp.workerCount += newWorkers
	fmt.Printf("Scaled up pool to %d workers\n", gp.workerCount)
}

// scaleDown removes workers from the pool
func (gp *GoroutinePool) scaleDown() {
	removeWorkers := min(gp.workerCount/8, gp.workerCount-gp.minWorkers)
	if removeWorkers <= 0 {
		return
	}

	// Send nil tasks to signal workers to stop
	for i := 0; i < removeWorkers; i++ {
		select {
		case gp.taskChan <- nil:
		default:
			break // Channel full, can't scale down now
		}
	}

	gp.workerCount -= removeWorkers
	fmt.Printf("Scaled down pool to %d workers\n", gp.workerCount)
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// GetPoolStats returns current pool performance statistics
func (gp *GoroutinePool) GetPoolStats() map[string]interface{} {
	gp.mu.RLock()
	defer gp.mu.RUnlock()

	return map[string]interface{}{
		"worker_count":     gp.workerCount,
		"min_workers":      gp.minWorkers,
		"max_workers":      gp.maxWorkers,
		"active_workers":   atomic.LoadInt64(&gp.activeWorkers),
		"queue_length":     len(gp.taskChan),
		"queue_capacity":   cap(gp.taskChan),
		"total_tasks":      atomic.LoadInt64(&gp.totalTasks),
		"completed_tasks":  atomic.LoadInt64(&gp.completedTasks),
		"rejected_tasks":   atomic.LoadInt64(&gp.rejectedTasks),
		"avg_task_time_ns": atomic.LoadInt64(&gp.avgTaskTime),
		"avg_task_time_ms": float64(atomic.LoadInt64(&gp.avgTaskTime)) / 1e6,
	}
}

// Stop stops the goroutine pool
func (gp *GoroutinePool) Stop() {
	close(gp.stopChan)
	gp.wg.Wait()
}

// GetStats returns pool statistics
func (gp *GoroutinePool) GetStats() map[string]interface{} {
	return map[string]interface{}{
		"worker_count":    gp.workerCount,
		"pending_tasks":   len(gp.taskChan),
		"available_slots": cap(gp.taskChan) - len(gp.taskChan),
	}
}

// PooledGoroutineServer implements single-threaded listener with pooled goroutine processing
// Pooled goroutines directly operate on cache memory
type PooledGoroutineServer struct {
	cache    *cache.Cache
	listener net.Listener
	pool     *GoroutinePool
	running  bool
	stopChan chan struct{}

	// Statistics
	totalConnections  uint64
	activeConnections uint64
	totalRequests     uint64
	totalResponses    uint64
	rejectedTasks     uint64
}

// NewPooledGoroutineServer creates a new pooled goroutine server
func NewPooledGoroutineServer(cache *cache.Cache, poolSize int) *PooledGoroutineServer {
	return &PooledGoroutineServer{
		cache:    cache,
		pool:     NewGoroutinePool(poolSize),
		stopChan: make(chan struct{}),
	}
}

// Start starts the pooled goroutine server
func (s *PooledGoroutineServer) Start(host, port string) error {
	listener, err := net.Listen("tcp", host+":"+port)
	if err != nil {
		return fmt.Errorf("failed to listen: %v", err)
	}
	s.listener = listener
	s.running = true

	// Start the goroutine pool
	s.pool.Start()

	fmt.Printf("Pooled-goroutine server started on %s:%s with %d workers\n", host, port, s.pool.workerCount)

	// Single-threaded accept loop
	for s.running {
		conn, err := s.listener.Accept()
		if err != nil {
			if s.running {
				fmt.Printf("Accept error: %v\n", err)
			}
			continue
		}

		atomic.AddUint64(&s.totalConnections, 1)
		atomic.AddUint64(&s.activeConnections, 1)

		// Submit connection to pool for direct cache memory operations
		task := &ConnectionTask{
			conn:   conn,
			server: s,
		}

		if !s.pool.SubmitTask(task) {
			// Pool is full, reject the connection
			atomic.AddUint64(&s.rejectedTasks, 1)
			atomic.AddUint64(&s.activeConnections, ^uint64(0)) // Decrement
			conn.Close()
			fmt.Printf("Connection rejected: pool is full\n")
		}
	}

	return nil
}

// processCommandDirect processes command with direct cache memory access (same as single goroutine)
func (s *PooledGoroutineServer) processCommandDirect(command string, authenticated *bool) string {
	parts := utils.ParseCommandWithQuotes(command)
	if len(parts) < 1 {
		return "ERROR invalid command format\n"
	}

	cmd := strings.ToUpper(parts[0])

	// Handle AUTH command
	if cmd == "AUTH" {
		return s.handleAuth(parts, authenticated)
	}

	// Check authentication
	authManager := s.cache.GetAuthManager()
	if authManager != nil && authManager.IsEnabled() && !*authenticated {
		return "ERROR authentication required\n"
	}

	// Parse TTL
	ttl, filteredParts, err := s.parseTTLFromParts(parts)
	if err != nil {
		return fmt.Sprintf("ERROR %v\n", err)
	}

	// Direct cache memory operations (no network overhead, no additional task queues)
	switch cmd {
	case "SET":
		if len(filteredParts) < 3 {
			return "ERROR SET requires key and value\n"
		}
		key := filteredParts[1]
		value := strings.Join(filteredParts[2:], " ")

		// Direct memory operation
		s.cache.Set(key, value, ttl)
		return "OK\n"

	case "SETS":
		// SET String array - simple space-separated values
		if len(filteredParts) < 3 {
			return "ERROR SETS requires key and at least one array element\n"
		}
		key := filteredParts[1]
		// All remaining parts become array elements
		array := filteredParts[2:]

		// Direct memory operation
		s.cache.Set(key, array, ttl)
		return "OK\n"

	case "SETX":
		// SET eXtended object - key-value pairs
		if len(filteredParts) < 4 {
			return "ERROR SETX requires key and at least one key-value pair\n"
		}
		if (len(filteredParts)-2)%2 != 0 {
			return "ERROR SETX requires even number of arguments for key-value pairs\n"
		}

		key := filteredParts[1]
		// Convert pairs to map: a b c d -> {a: b, c: d}
		object := make(map[string]string)
		for i := 2; i < len(filteredParts); i += 2 {
			object[filteredParts[i]] = filteredParts[i+1]
		}

		// Direct memory operation
		s.cache.Set(key, object, ttl)
		return "OK\n"

	case "GET":
		if len(filteredParts) < 2 {
			return "ERROR GET requires key\n"
		}
		key := filteredParts[1]

		// Direct memory operation
		value, exists := s.cache.Get(key)
		if !exists {
			return "NULL\n"
		}

		// Format output based on data type
		switch v := value.(type) {
		case string:
			// String: return as-is
			return fmt.Sprintf("%s\n", v)
		case []string:
			// Array: return as space-separated values in brackets
			return fmt.Sprintf("[%s]\n", strings.Join(v, " "))
		case map[string]string:
			// Object: return as JSON string
			jsonBytes, err := json.Marshal(v)
			if err != nil {
				return fmt.Sprintf("ERROR serializing object: %v\n", err)
			}
			return fmt.Sprintf("%s\n", string(jsonBytes))
		default:
			// Fallback for other types
			return fmt.Sprintf("%v\n", value)
		}

	case "DEL":
		if len(filteredParts) < 2 {
			return "ERROR DEL requires key\n"
		}
		key := filteredParts[1]

		// Direct memory operation
		deleted := s.cache.Delete(key)
		if deleted {
			return "OK\n"
		}
		return "NOT_FOUND\n"

	case "KEYS":
		pattern := "*"
		if len(filteredParts) > 1 {
			pattern = filteredParts[1]
		}

		// Direct memory operation
		keys := s.cache.Keys(pattern)
		if len(keys) == 0 {
			return "EMPTY\n"
		}
		return strings.Join(keys, " ") + "\n"

	case "FLUSHALL":
		// Direct memory operation
		s.cache.FlushAll()
		return "OK\n"

	default:
		return "ERROR unknown command\n"
	}
}

// handleAuth handles authentication (same as single goroutine)
func (s *PooledGoroutineServer) handleAuth(parts []string, authenticated *bool) string {
	if len(parts) != 2 {
		return "ERROR AUTH requires password\n"
	}

	password := parts[1]
	authManager := s.cache.GetAuthManager()

	if authManager != nil && authManager.IsEnabled() {
		valid, err := authManager.VerifyPassword(password)
		if err != nil {
			return fmt.Sprintf("ERROR authentication error: %v\n", err)
		} else if valid {
			*authenticated = true
			return "OK authenticated\n"
		} else {
			return "ERROR invalid password\n"
		}
	} else {
		*authenticated = true
		return "OK no authentication required\n"
	}
}

// parseTTLFromParts parses TTL from command parts (same as single goroutine)
func (s *PooledGoroutineServer) parseTTLFromParts(parts []string) (time.Duration, []string, error) {
	cmd := strings.ToUpper(parts[0])
	ttl := time.Duration(0)

	// Commands that don't support TTL
	if cmd == "DEL" || cmd == "GET" || cmd == "KEYS" || cmd == "FLUSHALL" {
		return ttl, parts, nil
	}

	// Handle TTL parameter: COMMAND key -t TTL_VALUE [other_params...]
	if len(parts) >= 4 && parts[2] == "-t" {
		ttlValue, err := utils.ParseTTL(parts[3])
		if err != nil {
			return 0, nil, fmt.Errorf("invalid ttl value: %v", err)
		}
		ttl = ttlValue
		// Remove -t and TTL value
		filteredParts := append([]string{parts[0], parts[1]}, parts[4:]...)
		return ttl, filteredParts, nil
	}

	return ttl, parts, nil
}

// Stop stops the server
func (s *PooledGoroutineServer) Stop() {
	s.running = false
	if s.listener != nil {
		s.listener.Close()
	}
	s.pool.Stop()
	close(s.stopChan)
}

// GetStats returns server statistics
func (s *PooledGoroutineServer) GetStats() map[string]interface{} {
	stats := map[string]interface{}{
		"total_connections":  atomic.LoadUint64(&s.totalConnections),
		"active_connections": atomic.LoadUint64(&s.activeConnections),
		"total_requests":     atomic.LoadUint64(&s.totalRequests),
		"total_responses":    atomic.LoadUint64(&s.totalResponses),
		"rejected_tasks":     atomic.LoadUint64(&s.rejectedTasks),
	}

	// Add pool stats
	poolStats := s.pool.GetStats()
	for k, v := range poolStats {
		stats["pool_"+k] = v
	}

	return stats
}
