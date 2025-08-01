package utils

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// parseTTL parses TTL string with time units (s, m, h, d, y)
// If no unit is specified, defaults to seconds
func ParseTTL(ttlStr string) (time.Duration, error) {
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
func ParseCommandWithQuotes(input string) []string {
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
