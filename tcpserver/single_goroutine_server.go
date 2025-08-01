package tcpserver

import (
	"bufio"
	"encoding/json"
	"fmt"
	"net"
	"strings"
	"sync/atomic"
	"time"

	"ant-cache/cache"
	"ant-cache/utils"
)

// SingleGoroutineServer implements single-threaded listener with one goroutine per connection
// Each connection gets its own goroutine that directly operates on cache memory
type SingleGoroutineServer struct {
	cache    *cache.Cache
	listener net.Listener
	running  bool
	stopChan chan struct{}

	// Statistics
	totalConnections  uint64
	activeConnections uint64
	totalRequests     uint64
	totalResponses    uint64
}

// NewSingleGoroutineServer creates a new single goroutine server
func NewSingleGoroutineServer(cache *cache.Cache) *SingleGoroutineServer {
	return &SingleGoroutineServer{
		cache:    cache,
		stopChan: make(chan struct{}),
	}
}

// Start starts the single goroutine server
func (s *SingleGoroutineServer) Start(host, port string) error {
	listener, err := net.Listen("tcp", host+":"+port)
	if err != nil {
		return fmt.Errorf("failed to listen: %v", err)
	}
	s.listener = listener
	s.running = true

	fmt.Printf("Single-goroutine server started on %s:%s\n", host, port)

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

		// Create a new goroutine for each connection
		// This goroutine directly operates on cache memory
		go s.handleConnection(conn)
	}

	return nil
}

// handleConnection handles a single connection with direct cache memory operations
func (s *SingleGoroutineServer) handleConnection(conn net.Conn) {
	defer func() {
		conn.Close()
		atomic.AddUint64(&s.activeConnections, ^uint64(0)) // Decrement
	}()

	// Set connection options for better performance
	if tcpConn, ok := conn.(*net.TCPConn); ok {
		tcpConn.SetNoDelay(true)
		tcpConn.SetKeepAlive(true)
		tcpConn.SetKeepAlivePeriod(30 * time.Second)
		tcpConn.SetReadBuffer(32768)  // 32KB read buffer
		tcpConn.SetWriteBuffer(32768) // 32KB write buffer
	}

	conn.SetReadDeadline(time.Now().Add(30 * time.Second))

	s.handleTextConnection(conn)
}

// handleTextConnection handles text protocol connection
func (s *SingleGoroutineServer) handleTextConnection(conn net.Conn) {
	scanner := bufio.NewScanner(conn)
	authenticated := false

	// Use larger buffer for better performance
	buf := make([]byte, 0, 64*1024)
	scanner.Buffer(buf, 1024*1024) // 1MB max token size

	for scanner.Scan() {
		line := scanner.Text()
		if line == "" {
			continue
		}

		atomic.AddUint64(&s.totalRequests, 1)

		// Reset read deadline
		conn.SetReadDeadline(time.Now().Add(30 * time.Second))

		// Process command directly in this goroutine (direct memory access)
		response := s.processCommandDirect(line, &authenticated)

		// Send response
		_, err := conn.Write([]byte(response))
		if err != nil {
			fmt.Printf("Failed to write response: %v\n", err)
			return
		}

		atomic.AddUint64(&s.totalResponses, 1)
	}

	if err := scanner.Err(); err != nil {
		fmt.Printf("Connection read error: %v\n", err)
	}
}

// processCommandDirect processes command with direct cache memory access
func (s *SingleGoroutineServer) processCommandDirect(command string, authenticated *bool) string {
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

	// Direct cache memory operations (no network overhead, no task queues)
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

// handleAuth handles authentication
func (s *SingleGoroutineServer) handleAuth(parts []string, authenticated *bool) string {
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

// parseTTLFromParts parses TTL from command parts
func (s *SingleGoroutineServer) parseTTLFromParts(parts []string) (time.Duration, []string, error) {
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
func (s *SingleGoroutineServer) Stop() {
	s.running = false
	if s.listener != nil {
		s.listener.Close()
	}
	close(s.stopChan)
}

// GetStats returns server statistics
func (s *SingleGoroutineServer) GetStats() map[string]interface{} {
	return map[string]interface{}{
		"total_connections":  atomic.LoadUint64(&s.totalConnections),
		"active_connections": atomic.LoadUint64(&s.activeConnections),
		"total_requests":     atomic.LoadUint64(&s.totalRequests),
		"total_responses":    atomic.LoadUint64(&s.totalResponses),
	}
}
