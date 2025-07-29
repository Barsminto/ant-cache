package cli

import (
	"ant-cache/cache"
	"ant-cache/tcpserver"
	"bufio"
	"fmt"
	"io"
	"net"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"
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

func StartInteractiveCLI(cache *cache.Cache, host string, port string, configPassword string) {
	// Check if authentication is required
	if configPassword != "" {
		fmt.Print("Password: ")
		reader := bufio.NewReader(os.Stdin)
		password, err := reader.ReadString('\n')
		if err != nil {
			fmt.Printf("Error reading password: %v\n", err)
			os.Exit(1)
		}
		password = strings.TrimSpace(password)

		if password != configPassword {
			fmt.Println("Authentication failed: invalid password")
			os.Exit(1)
		}
		fmt.Println("Authentication successful")
	}

	reader := bufio.NewReader(os.Stdin)
	fmt.Printf("Connected to ant-cache at %s:%s\n", host, port)
	fmt.Println("Type 'exit' to quit")

	fmt.Print("> ")
	for {
		input, err := reader.ReadString('\n')
		if err != nil {
			if err == io.EOF {
				break
			}
			fmt.Printf("Error reading input: %v\n", err)
			break
		}

		input = strings.TrimSpace(input)

		if input == "exit" || input == "quit" {
			break
		}

		parts := parseCommandWithQuotes(input)
		if len(parts) == 0 {
			fmt.Print("> ")
			continue
		}

		switch strings.ToUpper(parts[0]) {
		case "SET", "GET", "DEL", "SETS", "SETX", "SETNX", "SETSNX", "SETXNX", "KEYS", "FLUSHALL":
			handleCacheCommand(cache, parts)
			fmt.Print("> ")
		case "AUTH":
			handleAuthCommand(cache, parts)
			fmt.Print("> ")
		default:
			fmt.Println("ERROR: Unknown command")
			fmt.Print("> ")
		}
	}
}

func handleCacheCommand(cache *cache.Cache, parts []string) {
	// Reuse command handling logic from tcpserver
	handlers := tcpserver.CreateCommandHandlers()
	cmd := strings.ToUpper(parts[0])
	if handler, exists := handlers[cmd]; exists {
		// Handle TTL parameter - only support after key, not for DEL, GET, KEYS and FLUSHALL commands
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
					fmt.Printf("ERROR: Invalid TTL value: %v\n", err)
					return
				}
				ttl = ttlValue
				// Remove -t and TTL value, keep command and key, then add remaining params
				filteredParts = append([]string{parts[0], parts[1]}, parts[4:]...)
			} else {
				// No TTL parameter
				filteredParts = parts
			}
		}

		// KEYS and FLUSHALL commands can work without key name
		if (cmd == "KEYS" || cmd == "FLUSHALL") && len(filteredParts) < 1 {
			fmt.Println("ERROR: Invalid command format")
			return
		}

		// Other commands need at least 2 parameters
		if cmd != "KEYS" && cmd != "FLUSHALL" && len(filteredParts) < 2 {
			fmt.Println("ERROR: Missing key")
			return
		}

		// Create virtual connection
		conn := &fakeConn{reader: nil, writer: nil}

		// 对于KEYS和FLUSHALL命令，如果没有指定键名，传递空字符串
		var key string
		if len(filteredParts) >= 2 {
			key = filteredParts[1]
		}

		handler.Execute(conn, cache, key, filteredParts, ttl)

		// Automatically show prompt after command execution
		// fmt.Print("> ") // Moved to main loop handling
	} else {
		fmt.Println("ERROR: Unknown command")
		// fmt.Print("> ") // Moved to main loop handling
	}
}

type fakeConn struct {
	reader *io.PipeReader
	writer *io.PipeWriter
}

func (c *fakeConn) Read(b []byte) (n int, err error) { return 0, nil }
func (c *fakeConn) Write(b []byte) (n int, err error) {
	fmt.Print(string(b))
	return len(b), nil
}
func (c *fakeConn) Close() error                       { return nil }
func (c *fakeConn) LocalAddr() net.Addr                { return nil }
func (c *fakeConn) RemoteAddr() net.Addr               { return nil }
func (c *fakeConn) SetDeadline(t time.Time) error      { return nil }
func (c *fakeConn) SetReadDeadline(t time.Time) error  { return nil }
func (c *fakeConn) SetWriteDeadline(t time.Time) error { return nil }

func handleAuthCommand(cache *cache.Cache, parts []string) {
	authManager := cache.GetAuthManager()
	if authManager == nil || !authManager.IsEnabled() {
		fmt.Println("ERROR: Authentication is disabled")
		return
	}

	if len(parts) < 2 {
		fmt.Println("ERROR: AUTH command requires subcommand (setup, change)")
		return
	}

	switch strings.ToLower(parts[1]) {
	case "setup":
		if authManager.HasPassword() {
			fmt.Println("ERROR: Password already set. Use 'auth change' to change password.")
			return
		}
		if err := authManager.SetupPassword(); err != nil {
			fmt.Printf("ERROR: %v\n", err)
		}
	case "change":
		if !authManager.HasPassword() {
			fmt.Println("ERROR: No password set. Use 'auth setup' first.")
			return
		}

		currentPassword, err := authManager.PromptPassword("Enter current password: ")
		if err != nil {
			fmt.Printf("ERROR: %v\n", err)
			return
		}

		valid, err := authManager.VerifyPassword(currentPassword)
		if err != nil {
			fmt.Printf("ERROR: %v\n", err)
			return
		}

		if !valid {
			fmt.Println("ERROR: Invalid current password")
			return
		}

		newPassword, err := authManager.PromptPassword("Enter new password: ")
		if err != nil {
			fmt.Printf("ERROR: %v\n", err)
			return
		}

		if len(newPassword) < 6 {
			fmt.Println("ERROR: Password must be at least 6 characters long")
			return
		}

		confirmPassword, err := authManager.PromptPassword("Confirm new password: ")
		if err != nil {
			fmt.Printf("ERROR: %v\n", err)
			return
		}

		if newPassword != confirmPassword {
			fmt.Println("ERROR: Passwords do not match")
			return
		}

		if err := authManager.SetPassword(newPassword); err != nil {
			fmt.Printf("ERROR: Failed to change password: %v\n", err)
			return
		}

		fmt.Println("Password changed successfully!")
	default:
		fmt.Println("ERROR: Unknown auth subcommand. Available: setup, change")
	}
}
