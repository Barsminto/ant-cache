# Commands Reference

This document provides a complete reference for all Ant-Cache commands with syntax, examples, and usage patterns.

## Command Overview

Ant-Cache supports Redis-compatible commands with additional TTL functionality and advanced data types:

| Command | Purpose | Data Type | TTL Support | Status |
|---------|---------|-----------|-------------|---------|
| `SET` | Store string value | String | ✅ Yes | ✅ Implemented |
| `SETS` | Store array value | Array | ✅ Yes | ✅ Implemented |
| `SETX` | Store object value | Object | ✅ Yes | ✅ Implemented |
| `SETNX` | Store string only if key doesn't exist | String | ✅ Yes | ✅ Implemented |
| `SETSNX` | Store array only if key doesn't exist | Array | ✅ Yes | ✅ Implemented |
| `SETXNX` | Store object only if key doesn't exist | Object | ✅ Yes | ✅ Implemented |
| `GET` | Retrieve value by key | Any | ❌ No | ✅ Implemented |
| `DEL` | Delete key | Any | ❌ No | ✅ Implemented |
| `KEYS` | List keys by pattern | Any | ❌ No | ✅ Implemented |
| `FLUSHALL` | Clear all data | Any | ❌ No | ✅ Implemented |

## Connection

Connect to Ant-Cache using any TCP client:

```bash
# Using netcat
nc localhost 8890

# Using telnet
telnet localhost 8890

# Using Redis CLI (basic commands work)
redis-cli -h localhost -p 8890
```

## String Operations

### SET Command

Store a key-value pair with optional TTL.

**Syntax:**
```
SET key value
SET key -t TTL value
```

**Parameters:**
- `key`: String key name
- `value`: String value (can contain spaces)
- `TTL`: Time-to-live with units (s, m, h, d, y)

**Examples:**
```bash
# Basic set
SET username "john_doe"
# Response: OK

# Set with TTL (30 seconds)
SET session:abc123 -t 30s "active_session"
# Response: OK

# Set with different TTL units
SET cache:data -t 5m "temporary_data"    # 5 minutes
SET user:token -t 2h "auth_token"        # 2 hours
SET config:key -t 1d "daily_config"     # 1 day
SET backup:data -t 1y "yearly_backup"   # 1 year

# Set with quoted values containing spaces
SET message -t 1h "Hello, World! This is a test message."
# Response: OK
```

**TTL Units:**
- `s`: seconds
- `m`: minutes
- `h`: hours
- `d`: days
- `y`: years

### SETS Command

Store an array value with optional TTL using simple space-separated syntax.

**Syntax:**
```
SETS key element1 element2 element3 ...
SETS key -t TTL element1 element2 element3 ...
```

**Parameters:**
- `key`: String key name
- `element1 element2 ...`: Array elements (space-separated)
- `TTL`: Time-to-live with units (s, m, h, d, y)

**Examples:**
```bash
# Basic array set
SETS fruits apple banana cherry
# Response: OK

# Array with TTL (5 minutes)
SETS temp_list -t 5m one two three four
# Response: OK

# Array with quoted elements (spaces in values)
SETS messages "hello world" "good morning" "see you later"
# Response: OK

# Single element array
SETS single_item only_one
# Response: OK
```

**Important Notes:**
- Elements are space-separated, no JSON required
- Use quotes for elements containing spaces
- All array elements are stored as strings
- Retrieved arrays display as: `[item1 item2 item3]`

### SETX Command

Store an object (key-value map) with optional TTL using simple key-value pair syntax.

**Syntax:**
```
SETX key field1 value1 field2 value2 ...
SETX key -t TTL field1 value1 field2 value2 ...
```

**Parameters:**
- `key`: String key name
- `field1 value1 field2 value2 ...`: Key-value pairs (must be even number of arguments)
- `TTL`: Time-to-live with units (s, m, h, d, y)

**Examples:**
```bash
# Basic object set
SETX person name John age 30 city NYC
# Response: OK

# Object with TTL (2 hours)
SETX session -t 2h user_id 1001 role admin expires 2024-12-31
# Response: OK

# Object with quoted values (spaces in values)
SETX user_profile name "John Doe" email "john@example.com" role "admin user"
# Response: OK

# Simple configuration
SETX config debug true port 8890 host localhost
# Response: OK
```

**Important Notes:**
- Arguments after key must be even number (key-value pairs)
- Use quotes for values containing spaces
- All object values are stored as strings
- Retrieved objects display as: `map[field1:value1 field2:value2]`

### SETNX Command

Store a string value only if the key does not exist (atomic operation). Returns success only when the key is new.

**Syntax:**
```
SETNX key value
SETNX key -t TTL value
```

**Parameters:**
- `key`: String key name
- `value`: String value (can contain spaces)
- `TTL`: Time-to-live with units (s, m, h, d, y)

**Examples:**
```bash
# Set only if key doesn't exist
SETNX lock:resource1 "locked"
# Response: OK (key was set)

# Try to set existing key
SETNX lock:resource1 "locked_again"
# Response: NOT_SET (key already exists)

# SetNX with TTL
SETNX temp:session -t 1h "session_data"
# Response: OK

# Use for distributed locks
SETNX job:process1 "running"
# Response: OK

# Check if key exists before setting
GET lock:resource1
# Response: locked
```

**Important Notes:**
- Returns `OK` if key was successfully set
- Returns `NOT_SET` if key already exists
- Atomic operation - no race conditions
- Useful for distributed locks, rate limiting, initialization

### SETSNX Command

Store an array value only if the key does not exist (atomic operation).

**Syntax:**
```
SETSNX key element1 element2 element3 ...
SETSNX key -t TTL element1 element2 element3 ...
```

**Parameters:**
- `key`: String key name
- `element1 element2 ...`: Array elements (space-separated)
- `TTL`: Time-to-live with units (s, m, h, d, y)

**Examples:**
```bash
# Set array only if key doesn't exist
SETSNX initial_tags golang redis cache
# Response: OK

# Try to set existing key
SETSNX initial_tags python django flask
# Response: NOT_SET

# SetSNX with TTL
SETSNX temp_list -t 30m item1 item2 item3
# Response: OK

# Initialize configuration lists
SETSNX default_ports 80 443 8080
# Response: OK
```

**Important Notes:**
- Returns `OK` if key was successfully set
- Returns `NOT_SET` if key already exists
- All array elements are stored as strings
- Useful for initialization, configuration setup

### SETXNX Command

Store an object value only if the key does not exist (atomic operation).

**Syntax:**
```
SETXNX key field1 value1 field2 value2 ...
SETXNX key -t TTL field1 value1 field2 value2 ...
```

**Parameters:**
- `key`: String key name
- `field1 value1 field2 value2 ...`: Key-value pairs (must be even number of arguments)
- `TTL`: Time-to-live with units (s, m, h, d, y)

**Examples:**
```bash
# Set object only if key doesn't exist
SETXNX default_config debug false port 8890 host localhost
# Response: OK

# Try to set existing key
SETXNX default_config debug true port 9000
# Response: NOT_SET

# SetXNX with TTL
SETXNX initial_user -t 1h name admin role superuser
# Response: OK

# Initialize application settings
SETXNX app:settings theme dark language en timezone UTC
# Response: OK
```

**Important Notes:**
- Returns `OK` if key was successfully set
- Returns `NOT_SET` if key already exists
- Arguments after key must be even number (key-value pairs)
- Useful for configuration initialization, defaults setup

### GET Command

Retrieve the value associated with a key. Works with all data types (strings, arrays, objects).

**Syntax:**
```
GET key
```

**Examples:**
```bash
# Get string value
GET username
# Response: john_doe

# Get array value
GET users
# Response: [alice bob charlie]

# Get object value
GET user:1001
# Response: {"age":"30","city":"NYC","name":"John"}

# Get non-existent key
GET nonexistent
# Response: NULL

# Get expired key
GET expired_key
# Response: NULL
```

**Return Formats:**
- **String**: Plain text value
- **Array**: Space-separated values in brackets: `[item1 item2 item3]`
- **Object**: Standard JSON format: `{"key1":"value1","key2":"value2"}`

### DEL Command

Delete a key and its associated value.

**Syntax:**
```
DEL key
```

**Examples:**
```bash
# Delete existing key
DEL username
# Response: OK

# Delete non-existent key
DEL nonexistent
# Response: NOT_FOUND
```

## Utility Commands

### KEYS Command

List all keys matching a pattern.

**Syntax:**
```
KEYS pattern
```

**Pattern Matching:**
- `*`: Match any characters
- `?`: Match single character
- `[abc]`: Match any character in brackets
- `[a-z]`: Match any character in range

**Examples:**
```bash
# List all keys
KEYS *
# Response: key1 key2 key3

# List keys with prefix
KEYS user:*
# Response: user:1001 user:1002 user:admin

# List keys with suffix
KEYS *:token
# Response: auth:token session:token

# List keys with pattern
KEYS session:???
# Response: session:abc session:xyz

# No matching keys
KEYS nonexistent:*
# Response: EMPTY
```

### FLUSHALL Command

Remove all keys and values from the cache.

**Syntax:**
```
FLUSHALL
```

**Examples:**
```bash
# Clear all data
FLUSHALL
# Response: OK

# Verify all data is cleared
KEYS *
# Response: EMPTY
```

## Advanced Usage

### Working with Different Data Types

```bash
# String data
SET message -t 1h "Hello, World!"

# Array data (user lists, tags, etc.)
SETS tags -t 30m golang cache redis performance
SETS user_roles admin editor viewer

# Object data (structured information)
SETX user:profile -t 2h name John email john@example.com role admin
SETX app:config debug false timeout 30s max_connections 1000

# Mixed data types in same cache
SET simple_key "simple_value"
SETS list_key item1 item2 item3
SETX object_key field1 value1 field2 value2
```

### Data Type Use Cases

**Strings (`SET`)**:
- Simple key-value pairs
- Configuration values
- Session tokens
- Cached computed results

**Arrays (`SETS`)**:
- User lists and groups
- Tags and categories
- Menu items
- Search results

**Objects (`SETX`)**:
- User profiles
- Application configuration
- Structured data
- API responses

### Atomic Operations with NX Commands

The NX ("Not eXists") commands provide atomic operations that only succeed when the key doesn't exist. This is useful for:

- **Distributed locks**: Prevent multiple processes from accessing the same resource
- **Initialization**: Set default values only once
- **Rate limiting**: Create tokens only when they don't exist
- **Configuration setup**: Apply default configurations without overwriting existing ones

**Distributed Lock Example:**
```bash
# Process 1 attempts to acquire lock
SETNX lock:database_backup "process_1"
# Response: OK (lock acquired)

# Process 2 attempts to acquire same lock
SETNX lock:database_backup "process_2"
# Response: NOT_SET (lock already held)

# Release lock when done
DEL lock:database_backup
# Response: OK
```

**Initialization Example:**
```bash
# Set default configuration only once
SETXNX app:defaults debug false port 8890 max_connections 1000
# Response: OK (first time only)

# Subsequent attempts will fail
SETXNX app:defaults debug true port 9000
# Response: NOT_SET (already initialized)

# Check initialized configuration
GET app:defaults
# Response: {"debug":"false","port":"8890","max_connections":"1000"}
```

**Rate Limiting Example:**
```bash
# Create rate limit token (expires in 1 minute)
SETNX rate_limit:user123 -t 1m "1"
# Response: OK (first request)

# Subsequent requests within 1 minute
SETNX rate_limit:user123 -t 1m "1"
# Response: NOT_SET (rate limit active)

# After 1 minute, new token can be created
SETNX rate_limit:user123 -t 1m "1"
# Response: OK (rate limit reset)
```

### Batch Operations

```bash
# Multiple operations in sequence
SET counter -t 1h "1"
SETS active_users -t 30m alice bob charlie
SETX app_status -t 1d status active uptime 24h version 1.0

# Verify all operations
KEYS *
GET counter          # Response: 1
GET active_users     # Response: [alice bob charlie]
GET app_status       # Response: {"status":"active","uptime":"24h","version":"1.0"}
```

### TTL Examples

```bash
# Short-term cache (30 seconds)
SET temp:calculation -t 30s "result_12345"

# Session data (30 minutes)
SET session:user123 -t 30m "logged_in"

# Daily cache (24 hours)
SET daily:stats -t 1d "processed_1000_items"

# Long-term cache (1 week = 7 days)
SET weekly:report -t 7d "weekly_summary_data"
```

## Error Handling

### Common Error Responses

```bash
# Invalid command
INVALID_COMMAND
# Response: ERROR unknown command

# Missing parameters
SET
# Response: ERROR SET requires key and value

SETNX
# Response: ERROR SETNX requires key and value

SETSNX
# Response: ERROR SETSNX requires key and at least one array element

SETXNX
# Response: ERROR SETXNX requires key and at least one key-value pair

GET
# Response: ERROR GET requires key

DEL
# Response: ERROR DEL requires key

# Invalid TTL format
SET key -t invalid_ttl value
# Response: ERROR invalid ttl value: invalid TTL format: invalid_ttl

# SETS with no elements
SETS empty_key
# Response: ERROR SETS requires key and at least one array element

# SETX with odd number of arguments
SETX user name John age
# Response: ERROR SETX requires even number of arguments for key-value pairs

# SETX with insufficient arguments
SETX user name
# Response: ERROR SETX requires key and at least one key-value pair

# SETXNX with odd number of arguments
SETXNX config debug true port
# Response: ERROR SETXNX requires even number of arguments for key-value pairs

# SETXNX with insufficient arguments
SETXNX config debug
# Response: ERROR SETXNX requires key and at least one key-value pair
```

### Success Responses

```bash
# Successful operations
SET key value          # Response: OK
GET key               # Response: value
DEL key               # Response: OK
KEYS *                # Response: key1 key2 key3
FLUSHALL              # Response: OK

# Not found responses
GET nonexistent       # Response: NULL
DEL nonexistent       # Response: NOT_FOUND
KEYS nomatch:*        # Response: EMPTY
```

## Performance Considerations

### Efficient Key Naming

```bash
# Good: Hierarchical naming
SET user:1001:profile "data"
SET user:1001:settings "data"
SET session:abc123:data "data"

# Good: Consistent prefixes for KEYS operations
SET cache:user:1001 "data"
SET cache:session:abc123 "data"
SET temp:calculation:xyz "data"

# Avoid: Very long keys (impacts memory)
SET this_is_a_very_long_key_name_that_uses_too_much_memory "data"
```

### Batch Operations

```bash
# Efficient: Group related operations
SET user:1001:name "John"
SET user:1001:email "john@example.com"
SET user:1001:status "active"

# Less efficient: Many small unrelated operations
SET a "1"
SET b "2"
SET c "3"
```

### TTL Best Practices

```bash
# Use appropriate TTL for data lifecycle
SET user:session -t 30m "active"      # Session data
SET cache:expensive -t 1h "result"    # Computed results
SET config:daily -t 1d "settings"     # Daily configuration
SET backup:weekly -t 7d "data"        # Weekly backups

# Avoid very short TTLs for frequently accessed data
SET frequent:data -t 5s "value"       # May cause cache misses
```

## Integration Examples

### Application Integration

**Go Example:**
```go
package main

import (
    "fmt"
    "net"
    "bufio"
)

func main() {
    conn, err := net.Dial("tcp", "localhost:8890")
    if err != nil {
        panic(err)
    }
    defer conn.Close()
    
    // Set different data types
    fmt.Fprintf(conn, "SET simple_key simple_value\n")
    response, _ := bufio.NewReader(conn).ReadString('\n')
    fmt.Printf("SET response: %s", response)

    fmt.Fprintf(conn, "SETX user name John age 30 city NYC\n")
    response, _ = bufio.NewReader(conn).ReadString('\n')
    fmt.Printf("SETX response: %s", response)

    // Get values
    fmt.Fprintf(conn, "GET simple_key\n")
    response, _ = bufio.NewReader(conn).ReadString('\n')
    fmt.Printf("GET string: %s", response)  // simple_value

    fmt.Fprintf(conn, "GET user\n")
    response, _ = bufio.NewReader(conn).ReadString('\n')
    fmt.Printf("GET object: %s", response)  // {"age":"30","city":"NYC","name":"John"}
}
```

**Python Example:**
```python
import socket

def ant_cache_client():
    sock = socket.socket(socket.AF_INET, socket.SOCK_STREAM)
    sock.connect(('localhost', 8890))
    
    # Set different data types
    sock.send(b'SETS tags python golang redis\n')
    response = sock.recv(1024).decode().strip()
    print(f"SETS response: {response}")

    sock.send(b'SETX config -t 1h debug true port 8890\n')
    response = sock.recv(1024).decode().strip()
    print(f"SETX response: {response}")

    # Get values
    sock.send(b'GET tags\n')
    response = sock.recv(1024).decode().strip()
    print(f"GET array: {response}")  # [python golang redis]

    sock.send(b'GET config\n')
    response = sock.recv(1024).decode().strip()
    print(f"GET object: {response}")  # {"debug":"true","port":"8890"}
    
    sock.close()

ant_cache_client()
```

**Shell Script Example:**
```bash
#!/bin/bash

# Function to execute Ant-Cache command
execute_command() {
    echo "$1" | nc localhost 8890
}

# Set user data
execute_command "SET user:admin -t 2h admin_user"

# Get user data
result=$(execute_command "GET user:admin")
echo "User data: $result"

# List all user keys
users=$(execute_command "KEYS user:*")
echo "All users: $users"
```

## Command Line Interface

When using the built-in CLI mode:

```bash
# Start CLI mode
./ant-cache -cli

# CLI provides additional features:
> help                    # Show available commands
> SET key value          # Same syntax as TCP
> GET key                # Same syntax as TCP
> KEYS *                 # Same syntax as TCP
> exit                   # Exit CLI mode
```

## Next Steps

- Review [Installation Guide](INSTALLATION.md) for setup instructions
- Check [Performance Guide](PERFORMANCE.md) for optimization tips
- See configuration examples in the `configs/` directory
