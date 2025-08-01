package cli

import (
	"ant-cache/cache"
	"ant-cache/utils"
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"
	"time"
)

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

		parts := utils.ParseCommandWithQuotes(input)
		if len(parts) == 0 {
			fmt.Print("> ")
			continue
		}

		switch strings.ToUpper(parts[0]) {
		case "SET", "SETS", "SETX", "GET", "DEL", "KEYS", "FLUSHALL":
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
	cmd := strings.ToUpper(parts[0])

	// Parse TTL parameter
	ttl := time.Duration(0)
	var filteredParts []string

	// DEL, GET, KEYS and FLUSHALL commands don't support TTL parameter
	if cmd == "DEL" || cmd == "GET" || cmd == "KEYS" || cmd == "FLUSHALL" {
		filteredParts = parts
	} else {
		// Handle TTL parameter - only support after key
		// Format: COMMAND key -t TTL_VALUE [other_params...]
		if len(parts) >= 4 && len(parts) > 2 && parts[2] == "-t" {
			ttlValue, err := utils.ParseTTL(parts[3])
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

	// Execute commands directly
	switch cmd {
	case "SET":
		if len(filteredParts) < 3 {
			fmt.Println("ERROR: SET requires key and value")
			return
		}
		key := filteredParts[1]
		value := strings.Join(filteredParts[2:], " ")
		cache.Set(key, value, ttl)
		fmt.Println("OK")

	case "SETS":
		if len(filteredParts) < 3 {
			fmt.Println("ERROR: SETS requires key and at least one array element")
			return
		}
		key := filteredParts[1]
		// All remaining parts become array elements
		array := filteredParts[2:]

		cache.Set(key, array, ttl)
		fmt.Println("OK")

	case "SETX":
		if len(filteredParts) < 4 {
			fmt.Println("ERROR: SETX requires key and at least one key-value pair")
			return
		}
		if (len(filteredParts)-2)%2 != 0 {
			fmt.Println("ERROR: SETX requires even number of arguments for key-value pairs")
			return
		}

		key := filteredParts[1]
		// Convert pairs to map: a b c d -> {a: b, c: d}
		object := make(map[string]string)
		for i := 2; i < len(filteredParts); i += 2 {
			object[filteredParts[i]] = filteredParts[i+1]
		}

		cache.Set(key, object, ttl)
		fmt.Println("OK")

	case "GET":
		if len(filteredParts) < 2 {
			fmt.Println("ERROR: GET requires key")
			return
		}
		key := filteredParts[1]
		value, exists := cache.Get(key)
		if !exists {
			fmt.Println("NOT_FOUND")
		} else {
			// Format output based on data type
			switch v := value.(type) {
			case string:
				// String: return as-is
				fmt.Println(v)
			case []string:
				// Array: return as space-separated values in brackets
				fmt.Printf("[%s]\n", strings.Join(v, " "))
			case map[string]string:
				// Object: return as JSON string
				jsonBytes, err := json.Marshal(v)
				if err != nil {
					fmt.Printf("ERROR serializing object: %v\n", err)
				} else {
					fmt.Println(string(jsonBytes))
				}
			default:
				// Fallback for other types
				fmt.Println(value)
			}
		}

	case "DEL":
		if len(filteredParts) < 2 {
			fmt.Println("ERROR: DEL requires key")
			return
		}
		key := filteredParts[1]
		deleted := cache.Delete(key)
		if deleted {
			fmt.Println("OK")
		} else {
			fmt.Println("NOT_FOUND")
		}

	case "KEYS":
		pattern := "*"
		if len(filteredParts) > 1 {
			pattern = filteredParts[1]
		}
		keys := cache.Keys(pattern)
		if len(keys) == 0 {
			fmt.Println("No keys found")
		} else {
			fmt.Println(strings.Join(keys, " "))
		}

	case "FLUSHALL":
		cache.FlushAll()
		fmt.Println("OK")

	default:
		fmt.Println("ERROR: Unknown command")
	}
}

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
