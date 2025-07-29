# Commands Reference

Complete reference for all Ant-Cache commands with syntax, parameters, and examples.

## Command Syntax

All commands follow this format:
```
COMMAND key [-t TTL] [arguments...]
```

### TTL Parameter

The `-t` parameter specifies time-to-live and must come immediately after the key:
- `-t 30s` - 30 seconds
- `-t 5m` - 5 minutes
- `-t 2h` - 2 hours
- `-t 1d` - 1 day
- `-t 1y` - 1 year
- `-t 60` - 60 seconds (plain numbers default to seconds)

## String Commands

### SET - Set String Value

**Syntax:** `SET key [-t ttl] value`

**Description:** Sets a string value for the specified key.

**Parameters:**
- `key` - Key name
- `ttl` - Optional expiration time
- `value` - String value to store

**Returns:** `OK`

**Examples:**
```bash
SET user:name "John Doe"
SET session:abc123 -t 30m "session_data"
SET counter -t 3600 "100"
SET message "Hello World"
```

### SETNX - Set String If Not Exists

**Syntax:** `SETNX key [-t ttl] value`

**Description:** Sets a string value only if the key does not already exist. This is an atomic operation.

**Parameters:**
- `key` - Key name
- `ttl` - Optional expiration time
- `value` - String value to store

**Returns:** 
- `1` - Key was set successfully
- `0` - Key already exists, not set

**Examples:**
```bash
SETNX lock:resource1 -t 30s "process_1"
# Returns: 1 (lock acquired)

SETNX lock:resource1 "process_2"
# Returns: 0 (lock already held)
```

## Array Commands

### SETS - Set Array Value

**Syntax:** `SETS key [-t ttl] item1 [item2 item3 ...]`

**Description:** Sets an array of string values for the specified key.

**Parameters:**
- `key` - Key name
- `ttl` - Optional expiration time
- `item1, item2, ...` - Array elements

**Returns:** `OK`

**Examples:**
```bash
SETS user:tags work important urgent
SETS shopping:list -t 1d milk bread eggs cheese
SETS permissions read write execute
```

### SETSNX - Set Array If Not Exists

**Syntax:** `SETSNX key [-t ttl] item1 [item2 item3 ...]`

**Description:** Sets an array value only if the key does not already exist. This is an atomic operation.

**Parameters:**
- `key` - Key name
- `ttl` - Optional expiration time
- `item1, item2, ...` - Array elements

**Returns:**
- `1` - Key was set successfully
- `0` - Key already exists, not set

**Examples:**
```bash
SETSNX user:permissions -t 1h read write admin
# Returns: 1 (permissions set)

SETSNX user:permissions read
# Returns: 0 (permissions already exist)
```

## Object Commands

### SETX - Set Object Value

**Syntax:** `SETX key [-t ttl] field1 value1 [field2 value2 ...]`

**Description:** Sets an object (key-value pairs) for the specified key.

**Parameters:**
- `key` - Key name
- `ttl` - Optional expiration time
- `field1 value1, field2 value2, ...` - Field-value pairs

**Returns:** `OK`

**Examples:**
```bash
SETX user:profile name "John" age 30 email "john@example.com"
SETX config:app -t 1h debug true port 8080 host "localhost"
SETX session:data -t 30m user_id 1001 role admin last_seen "2025-01-01"
```

### SETXNX - Set Object If Not Exists

**Syntax:** `SETXNX key [-t ttl] field1 value1 [field2 value2 ...]`

**Description:** Sets an object value only if the key does not already exist. This is an atomic operation.

**Parameters:**
- `key` - Key name
- `ttl` - Optional expiration time
- `field1 value1, field2 value2, ...` - Field-value pairs

**Returns:**
- `1` - Key was set successfully
- `0` - Key already exists, not set

**Examples:**
```bash
SETXNX user:settings -t 24h theme dark language en timezone "UTC"
# Returns: 1 (settings created)

SETXNX user:settings theme light
# Returns: 0 (settings already exist)
```

## Query Commands

### GET - Get Value

**Syntax:** `GET key1 [key2 key3 ...]`

**Description:** Retrieves the value(s) for the specified key(s).

**Parameters:**
- `key1, key2, ...` - One or more key names

**Returns:**
- Single key: Returns the value directly
- Multiple keys: Returns JSON object with key-value pairs
- Non-existent key: `NOT_FOUND`

**Examples:**
```bash
# Single key
GET user:name
# Returns: John Doe

# Multiple keys
GET user:name user:age user:email
# Returns: {"user:age":"30","user:email":"john@example.com","user:name":"John Doe"}

# Array value
GET user:tags
# Returns: ["work","important","urgent"]

# Object value
GET user:profile
# Returns: {"age":"30","email":"john@example.com","name":"John"}

# Non-existent key
GET missing:key
# Returns: NOT_FOUND
```

## Management Commands

### DEL - Delete Keys

**Syntax:** `DEL key1 [key2 key3 ...]`

**Description:** Deletes one or more keys from the cache.

**Parameters:**
- `key1, key2, ...` - One or more key names to delete

**Returns:** Number of keys successfully deleted

**Examples:**
```bash
# Delete single key
DEL user:name
# Returns: 1

# Delete multiple keys
DEL user:name user:age user:email
# Returns: 2 (if one key didn't exist)

# Delete non-existent key
DEL missing:key
# Returns: 0
```

### KEYS - List All Keys

**Syntax:** `KEYS`

**Description:** Lists all keys currently stored in the cache with their data types.

**Parameters:** None

**Returns:** List of keys in format `keyname (type)`

**Examples:**
```bash
KEYS
# Returns:
# user:name (string)
# user:tags (array)
# user:profile (object)
# session:abc123 (string)
```

### FLUSHALL - Clear All Data

**Syntax:** `FLUSHALL`

**Description:** Removes all keys from the cache. Use with extreme caution!

**Parameters:** None

**Returns:** `OK n keys deleted` where n is the number of deleted keys

**Examples:**
```bash
FLUSHALL
# Returns: OK 15 keys deleted
```

## Authentication Commands

### AUTH - Authenticate

**Syntax:** `AUTH password`

**Description:** Authenticates with the server using the provided password. Only required if authentication is enabled in the configuration.

**Parameters:**
- `password` - The authentication password

**Returns:**
- `OK authenticated` - Authentication successful
- `ERROR invalid password` - Wrong password
- `OK no authentication required` - Authentication not enabled

**Examples:**
```bash
AUTH mypassword
# Returns: OK authenticated
```

## Response Formats

### Success Responses

- `OK` - Command executed successfully
- `1` / `0` - Boolean result (for NX commands)
- `Number` - Numeric result (e.g., count of deleted keys)
- `String` - Direct string value
- `JSON Array` - Array data: `["item1","item2","item3"]`
- `JSON Object` - Object data: `{"field1":"value1","field2":"value2"}`

### Error Responses

All errors start with `ERROR`:

- `ERROR invalid command format` - Syntax error in command
- `ERROR invalid ttl value` - Invalid TTL format
- `ERROR missing key` - Key parameter missing
- `ERROR unknown command` - Command not recognized
- `ERROR authentication required` - Must authenticate first
- `ERROR invalid password` - Wrong password provided

## Command Examples by Use Case

### Session Management

```bash
# Create session
SETX session:abc123 -t 30m user_id 1001 username "john" role "admin"

# Get session
GET session:abc123

# Update session (extend TTL)
SETX session:abc123 -t 30m user_id 1001 username "john" role "admin" last_activity "2025-01-01T10:00:00Z"

# Delete session
DEL session:abc123
```

### Caching API Responses

```bash
# Cache API response
SET api:users:list -t 5m "[{\"id\":1,\"name\":\"John\"},{\"id\":2,\"name\":\"Jane\"}]"

# Get cached response
GET api:users:list

# Clear cache
DEL api:users:list
```

### Distributed Locking

```bash
# Acquire lock
SETNX lock:payment:order123 -t 30s "process_1"
# Returns: 1 (acquired) or 0 (already locked)

# Check lock
GET lock:payment:order123

# Release lock
DEL lock:payment:order123
```

### Configuration Storage

```bash
# Store configuration
SETX app:config -t 1h 
  database_host "localhost" 
  database_port 5432 
  debug_mode true 
  max_connections 100

# Get configuration
GET app:config

# Update single config value (requires full reset)
SETX app:config -t 1h 
  database_host "localhost" 
  database_port 5432 
  debug_mode false 
  max_connections 100
```

### User Permissions

```bash
# Set user permissions
SETS user:1001:permissions -t 2h read write admin

# Check permissions
GET user:1001:permissions

# Add permissions (requires full reset)
SETS user:1001:permissions -t 2h read write admin delete
```

## Best Practices

### 1. TTL Parameter Position

Always place `-t` immediately after the key:

```bash
# Correct
SET mykey -t 30s myvalue
SETS myarray -t 1h item1 item2
SETX myobject -t 2h field1 value1

# Incorrect
SET mykey myvalue -t 30s
SETS myarray item1 item2 -t 1h
```

### 2. Atomic Operations

Use NX commands for race-condition-free operations:

```bash
# Safe lock acquisition
SETNX lock:resource -t 30s "owner"

# Safe initialization
SETSNX user:permissions read write
SETXNX app:settings theme dark lang en
```

### 3. Key Naming

Use consistent, hierarchical naming:

```bash
# Good
user:1001:profile
session:abc123
cache:api:users
lock:resource:payment

# Avoid
user1001profile
randomkey
temp123
```

### 4. Error Handling

Always check return values:

```bash
# Check SET result
SET mykey myvalue
# Expected: OK

# Check SETNX result
SETNX lock:resource "owner"
# Check: 1 (success) or 0 (exists)

# Handle NOT_FOUND
GET missing:key
# Handle: NOT_FOUND
```

## Performance Notes

- **Batch Operations**: Use multi-key GET when possible
- **TTL Efficiency**: Avoid very short TTLs (< 1s) unless necessary
- **Key Length**: Keep key names reasonably short
- **Value Size**: Keep individual values under 1MB for best performance
- **Connection Reuse**: Reuse connections in high-traffic applications

For more performance tips, see the [Performance Report](PERFORMANCE.md).
