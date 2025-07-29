package tcpserver

import (
	"bufio"
	"encoding/json"
	"fmt"
	"net"
	"regexp"
	"strconv"
	"strings"
	"time"

	"ant-cache/cache"
)

// parseTTL parses TTL string with time units (s, m, h, d, y)
// If no unit is specified, defaults to seconds
func parseTTL(ttlStr string) (time.Duration, error) {
	if ttlStr == "" {
		return 0, nil
	}

	// Check if it's just a number (default to seconds)
	if num, err := strconv.Atoi(ttlStr); err == nil {
		return time.Duration(num) * time.Second, nil
	}

	// Parse with time units
	re := regexp.MustCompile(`^(\d+)([smhdy])$`)
	matches := re.FindStringSubmatch(strings.ToLower(ttlStr))
	if len(matches) != 3 {
		return 0, fmt.Errorf("invalid TTL format: %s", ttlStr)
	}

	num, err := strconv.Atoi(matches[1])
	if err != nil {
		return 0, fmt.Errorf("invalid number in TTL: %s", matches[1])
	}

	unit := matches[2]
	switch unit {
	case "s":
		return time.Duration(num) * time.Second, nil
	case "m":
		return time.Duration(num) * time.Minute, nil
	case "h":
		return time.Duration(num) * time.Hour, nil
	case "d":
		return time.Duration(num) * 24 * time.Hour, nil
	case "y":
		return time.Duration(num) * 365 * 24 * time.Hour, nil
	default:
		return 0, fmt.Errorf("invalid time unit: %s", unit)
	}
}

// parseCommandWithQuotes parses a command line respecting quoted strings
func parseCommandWithQuotes(input string) []string {
	var parts []string
	var current strings.Builder
	inQuotes := false
	quoteChar := byte(0)

	input = strings.TrimSpace(input)

	for i := 0; i < len(input); i++ {
		char := input[i]

		if !inQuotes {
			// Not in quotes
			if char == '"' || char == '\'' {
				// Start of quoted string
				inQuotes = true
				quoteChar = char
			} else if char == ' ' || char == '\t' {
				// Whitespace - end current part
				if current.Len() > 0 {
					parts = append(parts, current.String())
					current.Reset()
				}
				// Skip multiple whitespace
				for i+1 < len(input) && (input[i+1] == ' ' || input[i+1] == '\t') {
					i++
				}
			} else {
				// Regular character
				current.WriteByte(char)
			}
		} else {
			// In quotes
			if char == quoteChar {
				// End of quoted string
				inQuotes = false
				quoteChar = 0
			} else if char == '\\' && i+1 < len(input) {
				// Escape sequence
				i++
				switch input[i] {
				case 'n':
					current.WriteByte('\n')
				case 't':
					current.WriteByte('\t')
				case 'r':
					current.WriteByte('\r')
				case '\\':
					current.WriteByte('\\')
				case '"':
					current.WriteByte('"')
				case '\'':
					current.WriteByte('\'')
				default:
					// Unknown escape, keep both characters
					current.WriteByte('\\')
					current.WriteByte(input[i])
				}
			} else {
				// Regular character in quotes
				current.WriteByte(char)
			}
		}
	}

	// Add final part
	if current.Len() > 0 {
		parts = append(parts, current.String())
	}

	return parts
}

const (
	SET    = "SET"
	GET    = "GET"
	DEL    = "DEL"
	SETS   = "SETS"
	SETX   = "SETX"
	SETNX  = "SETNX"
	SETSNX = "SETSNX"
	SETXNX = "SETXNX"
	// STATS command removed
	KEYS     = "KEYS"
	FLUSHALL = "FLUSHALL"
)

func Start(cache *cache.Cache, port string) error {
	ln, err := net.Listen("tcp", ":"+port)
	if err != nil {
		return err
	}
	defer ln.Close()

	for {
		conn, err := ln.Accept()
		if err != nil {
			continue
		}
		go handleConnection(conn, cache)
	}
}

type CommandHandler interface {
	Execute(conn net.Conn, cache *cache.Cache, key string, parts []string, ttl time.Duration)
}

type SetCommand struct{}

func (c *SetCommand) Execute(conn net.Conn, cache *cache.Cache, key string, parts []string, ttl time.Duration) {
	if len(parts) < 3 {
		fmt.Fprintf(conn, "ERROR invalid SET command\n")
		return
	}
	value := parts[2]
	cache.Set(key, value, ttl)
	fmt.Fprintf(conn, "OK\n")
}

type SetsCommand struct{}

func (c *SetsCommand) Execute(conn net.Conn, cache *cache.Cache, key string, parts []string, ttl time.Duration) {
	if len(parts) < 3 {
		fmt.Fprintf(conn, "ERROR invalid SETS command\n")
		return
	}
	values := parts[2:]
	cache.Set(key, values, ttl)
	fmt.Fprintf(conn, "OK\n")
}

type SetxCommand struct{}

func (c *SetxCommand) Execute(conn net.Conn, cache *cache.Cache, key string, parts []string, ttl time.Duration) {
	if len(parts) < 3 {
		fmt.Fprintf(conn, "ERROR invalid SETX command\n")
		return
	}
	obj := make(map[string]string)
	for i := 2; i < len(parts); i += 2 {
		if i+1 < len(parts) {
			obj[parts[i]] = parts[i+1]
		} else {
			obj[parts[i]] = ""
		}
	}
	cache.Set(key, obj, ttl)
	fmt.Fprintf(conn, "OK\n")
}

type SetnxCommand struct{}

func (c *SetnxCommand) Execute(conn net.Conn, cache *cache.Cache, key string, parts []string, ttl time.Duration) {
	if len(parts) < 3 {
		fmt.Fprintf(conn, "ERROR invalid SETNX command\n")
		return
	}
	value := parts[2]
	if cache.SetNX(key, value, ttl) {
		fmt.Fprintf(conn, "1\n")
	} else {
		fmt.Fprintf(conn, "0\n")
	}
}

type SetsnxCommand struct{}

func (c *SetsnxCommand) Execute(conn net.Conn, cache *cache.Cache, key string, parts []string, ttl time.Duration) {
	if len(parts) < 3 {
		fmt.Fprintf(conn, "ERROR invalid SETSNX command\n")
		return
	}
	values := parts[2:]
	if cache.SetNX(key, values, ttl) {
		fmt.Fprintf(conn, "1\n")
	} else {
		fmt.Fprintf(conn, "0\n")
	}
}

type SetxnxCommand struct{}

func (c *SetxnxCommand) Execute(conn net.Conn, cache *cache.Cache, key string, parts []string, ttl time.Duration) {
	if len(parts) < 3 {
		fmt.Fprintf(conn, "ERROR invalid SETXNX command\n")
		return
	}
	obj := make(map[string]string)
	for i := 2; i < len(parts); i += 2 {
		if i+1 < len(parts) {
			obj[parts[i]] = parts[i+1]
		} else {
			obj[parts[i]] = ""
		}
	}
	if cache.SetNX(key, obj, ttl) {
		fmt.Fprintf(conn, "1\n")
	} else {
		fmt.Fprintf(conn, "0\n")
	}
}

type GetCommand struct{}

func (c *GetCommand) Execute(conn net.Conn, cache *cache.Cache, key string, parts []string, ttl time.Duration) {
	// Support multiple keys: GET key1 key2 key3...
	if len(parts) < 2 {
		fmt.Fprintf(conn, "ERROR: GET requires at least one key\n")
		return
	}

	keys := parts[1:] // All keys from the command

	if len(keys) == 1 {
		// Single key - format output properly
		value, found := cache.Get(keys[0])
		if !found {
			fmt.Fprintf(conn, "NOT_FOUND\n")
			return
		}

		// Format the output based on type
		c.formatSingleValue(conn, value)
	} else {
		// Multiple keys
		results := cache.GetMultiple(keys)
		if len(results) == 0 {
			fmt.Fprintf(conn, "NOT_FOUND\n")
			return
		}

		// Format output as JSON for multiple keys
		jsonData, err := json.MarshalIndent(results, "", "  ")
		if err != nil {
			fmt.Fprintf(conn, "ERROR: Failed to format results\n")
			return
		}
		fmt.Fprintf(conn, "%s\n", jsonData)
	}
}

// formatSingleValue formats a single value for output
func (c *GetCommand) formatSingleValue(conn net.Conn, value interface{}) {
	switch v := value.(type) {
	case string:
		// String values - output directly
		fmt.Fprintf(conn, "%s\n", v)
	case []string:
		// Array values - output as JSON array
		jsonData, err := json.Marshal(v)
		if err != nil {
			fmt.Fprintf(conn, "ERROR: Failed to format array\n")
			return
		}
		fmt.Fprintf(conn, "%s\n", jsonData)
	case map[string]string:
		// Object values - output as formatted JSON
		jsonData, err := json.MarshalIndent(v, "", "  ")
		if err != nil {
			fmt.Fprintf(conn, "ERROR: Failed to format object\n")
			return
		}
		fmt.Fprintf(conn, "%s\n", jsonData)
	default:
		// Other types - try to format as JSON
		jsonData, err := json.MarshalIndent(v, "", "  ")
		if err != nil {
			// Fallback to string representation
			fmt.Fprintf(conn, "%v\n", v)
		} else {
			fmt.Fprintf(conn, "%s\n", jsonData)
		}
	}
}

// GetsCommand removed - use GET instead

// CreateCommandHandlers returns a map of all available command handlers
func CreateCommandHandlers() map[string]CommandHandler {
	return map[string]CommandHandler{
		SET:    &SetCommand{},
		SETS:   &SetsCommand{},
		SETX:   &SetxCommand{},
		SETNX:  &SetnxCommand{},
		SETSNX: &SetsnxCommand{},
		SETXNX: &SetxnxCommand{},
		GET:    &GetCommand{},
		DEL:    &DelCommand{},
		// STATS command removed
		KEYS:     &KeysCommand{},
		FLUSHALL: &FlushAllCommand{},
	}
}

// GetxCommand removed - use GET instead

type DelCommand struct{}

func (c *DelCommand) Execute(conn net.Conn, cache *cache.Cache, key string, parts []string, ttl time.Duration) {
	// Support multiple keys: DEL key1 key2 key3 ...
	// Skip the command name (parts[0])
	keys := parts[1:]

	if len(keys) == 0 {
		fmt.Fprintf(conn, "ERROR missing key\n")
		return
	}

	deletedCount := 0
	for _, k := range keys {
		if cache.Delete(k) {
			deletedCount++
		}
	}

	fmt.Fprintf(conn, "%d\n", deletedCount)
}

// StatsCommand removed

type KeysCommand struct{}

func (c *KeysCommand) Execute(conn net.Conn, cache *cache.Cache, key string, parts []string, ttl time.Duration) {
	keys := cache.GetAllKeys()
	if len(keys) == 0 {
		fmt.Fprintf(conn, "No keys found\n")
		return
	}

	for _, keyInfo := range keys {
		keyName := keyInfo["key"].(string)
		keyType := keyInfo["type"].(string)
		fmt.Fprintf(conn, "%s (%s)\n", keyName, keyType)
	}
}

type FlushAllCommand struct{}

func (c *FlushAllCommand) Execute(conn net.Conn, cache *cache.Cache, key string, parts []string, ttl time.Duration) {
	count := cache.FlushAll()
	fmt.Fprintf(conn, "OK %d keys deleted\n", count)
}

func handleConnection(conn net.Conn, cache *cache.Cache) {
	defer func(conn net.Conn) {
		_ = conn.Close()
	}(conn)

	// Check if authentication is required
	authManager := cache.GetAuthManager()
	authenticated := false
	if authManager == nil || !authManager.IsEnabled() {
		authenticated = true // No auth required
	}

	handlers := CreateCommandHandlers()

	scanner := bufio.NewScanner(conn)
	for scanner.Scan() {
		text := strings.TrimSpace(scanner.Text())
		parts := parseCommandWithQuotes(text)
		if len(parts) < 1 {
			fmt.Fprintf(conn, "ERROR invalid command format\n")
			continue
		}

		cmd := strings.ToUpper(parts[0])

		// Handle AUTH command
		if cmd == "AUTH" {
			if len(parts) != 2 {
				fmt.Fprintf(conn, "ERROR AUTH requires password\n")
				continue
			}
			password := parts[1]
			if authManager != nil && authManager.IsEnabled() {
				valid, err := authManager.VerifyPassword(password)
				if err != nil {
					fmt.Fprintf(conn, "ERROR authentication error: %v\n", err)
				} else if valid {
					authenticated = true
					fmt.Fprintf(conn, "OK authenticated\n")
				} else {
					fmt.Fprintf(conn, "ERROR invalid password\n")
				}
			} else {
				authenticated = true
				fmt.Fprintf(conn, "OK no authentication required\n")
			}
			continue
		}

		// Check authentication for other commands
		if !authenticated {
			fmt.Fprintf(conn, "ERROR authentication required\n")
			continue
		}

		// Parse TTL - support after key, not for DEL, GET, KEYS and FLUSHALL commands
		ttl := time.Duration(0)
		var filteredParts []string

		// DEL, GET, KEYS and FLUSHALL commands don't support TTL parameter
		if cmd == "DEL" || cmd == "GET" || cmd == "KEYS" || cmd == "FLUSHALL" {
			filteredParts = parts
		} else {
			// Handle TTL parameter - only support after key
			// Format: COMMAND key -t TTL_VALUE [other_params...]
			if len(parts) >= 4 && len(parts) > 2 && parts[2] == "-t" {
				ttlValue, err := parseTTL(parts[3])
				if err != nil {
					fmt.Fprintf(conn, "ERROR invalid ttl value: %v\n", err)
					continue
				}
				ttl = ttlValue
				// Remove -t and TTL value, keep command and key, then add remaining params
				filteredParts = append([]string{parts[0], parts[1]}, parts[4:]...)
			} else {
				// No TTL parameter
				filteredParts = parts
			}
		}

		// Command format validation
		if len(filteredParts) < 1 {
			fmt.Fprintf(conn, "ERROR invalid command format\n")
			continue
		}

		// KEYS and FLUSHALL commands don't need a key parameter
		// GET command can have multiple keys, so we check differently
		if len(filteredParts) < 2 && cmd != "KEYS" && cmd != "FLUSHALL" {
			fmt.Fprintf(conn, "ERROR invalid command format\n")
			continue
		}

		// Special check for GET command - needs at least one key
		if cmd == "GET" && len(filteredParts) < 2 {
			fmt.Fprintf(conn, "ERROR GET requires at least one key\n")
			continue
		}

		var key string
		if len(filteredParts) >= 2 {
			key = filteredParts[1]
		}

		if handler, exists := handlers[cmd]; exists {
			handler.Execute(conn, cache, key, filteredParts, ttl)
		} else {
			fmt.Fprintf(conn, "ERROR unknown command\n")
		}
	}
}
