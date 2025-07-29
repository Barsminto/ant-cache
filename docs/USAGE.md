# Usage Guide

This guide covers how to use Ant-Cache effectively, including basic operations, data types, and best practices.

## Getting Started

### Starting Ant-Cache

```bash
# Interactive CLI mode
./ant-cache -cli

# Server mode (background)
./ant-cache
```

### Basic Connection

```bash
# Connect via CLI
./ant-cache -cli

# Connect via telnet
telnet localhost 8890

# Connect via netcat
nc localhost 8890
```

## Data Types

Ant-Cache supports three data types: String, Array, and Object.

### String Operations

```bash
# Set string value
SET key value
SET user:name "John Doe"

# Set with expiration
SET session:abc123 -t 30m "session_data"
SET temp:key -t 60s "temporary_value"

# Set only if not exists (atomic)
SETNX lock:resource1 -t 30s "process_1"

# Get string value
GET user:name
# Returns: John Doe

# Get multiple keys
GET user:name user:email user:age
# Returns: {"user:age":"30","user:email":"john@example.com","user:name":"John Doe"}
```

### Array Operations

```bash
# Set array
SETS tags work important urgent
SETS user:permissions -t 1h read write admin

# Set array only if not exists
SETSNX user:roles -t 2h user admin

# Get array
GET tags
# Returns: ["work","important","urgent"]
```

### Object Operations

```bash
# Set object (key-value pairs)
SETX user:profile name "John" age 30 email "john@example.com"
SETX config:app -t 1h debug true port 8080

# Set object only if not exists
SETXNX user:settings -t 24h theme dark language en

# Get object
GET user:profile
# Returns: {"age":"30","email":"john@example.com","name":"John"}
```

## TTL (Time To Live)

### Time Formats

Ant-Cache supports flexible time formats:

- `30s` - 30 seconds
- `5m` - 5 minutes
- `2h` - 2 hours
- `1d` - 1 day
- `1y` - 1 year
- `60` - 60 seconds (plain numbers default to seconds)

### TTL Examples

```bash
# Short-term cache (30 seconds)
SET api:cache:user:1 -t 30s "cached_user_data"

# Session data (30 minutes)
SET session:abc123 -t 30m "session_info"

# Daily cache (1 day)
SET daily:stats -t 1d "statistics_data"

# Configuration cache (1 hour)
SETX app:config -t 1h database_url "localhost" port 5432
```

## Management Commands

### Key Management

```bash
# List all keys
KEYS
# Returns:
# user:name (string)
# user:tags (array)
# user:profile (object)

# Delete single key
DEL user:name
# Returns: 1

# Delete multiple keys
DEL user:name user:email user:age
# Returns: 2 (number of deleted keys)

# Clear all data (use with caution!)
FLUSHALL
# Returns: OK 15 keys deleted
```

## Authentication

### Using Authentication

If authentication is enabled in config.json:

```bash
# CLI mode - you'll be prompted for password
./ant-cache -cli
Password: [enter your password]
Authentication successful
ant-cache>
```

### TCP Connection Authentication

```bash
# Connect via telnet
telnet localhost 8890
AUTH your_password
# Returns: OK authenticated

# Now you can use commands
SET test value
```

## Common Use Cases

### 1. API Response Caching

```bash
# Cache API response for 5 minutes
SET api:users:list -t 5m "[{\"id\":1,\"name\":\"John\"},{\"id\":2,\"name\":\"Jane\"}]"

# Retrieve cached response
GET api:users:list
```

### 2. Session Management

```bash
# Store session data for 30 minutes
SETX session:abc123 -t 30m user_id 1001 username "john" role "admin"

# Retrieve session
GET session:abc123

# Extend session (set again with new TTL)
SETX session:abc123 -t 30m user_id 1001 username "john" role "admin"
```

### 3. Rate Limiting

```bash
# Track API calls per user (1 minute window)
SET rate:user:1001 -t 60s "1"

# Increment counter (you'd need to implement this logic in your app)
# If key exists, increment; if not, start at 1
```

### 4. Distributed Locking

```bash
# Acquire lock (atomic operation)
SETNX lock:payment:order123 -t 30s "process_1"
# Returns: 1 (lock acquired) or 0 (lock already held)

# Release lock
DEL lock:payment:order123
```

### 5. Configuration Caching

```bash
# Cache application configuration
SETX app:config -t 1h 
  database_host "localhost" 
  database_port 5432 
  redis_url "redis://localhost:6379"
  debug_mode true

# Retrieve configuration
GET app:config
```

### 6. Temporary Data Storage

```bash
# Store temporary processing results
SETS processing:batch123 -t 10m "item1" "item2" "item3"

# Store temporary user uploads
SET upload:temp:abc123 -t 1h "/tmp/upload_abc123.jpg"
```

## Best Practices

### 1. Key Naming Conventions

Use consistent, hierarchical naming:

```bash
# Good examples
user:1001:profile
session:abc123
cache:api:users:list
lock:resource:payment
config:app:database

# Avoid
user1001profile
sessionabc123
random_key_name
```

### 2. TTL Strategy

- **Session data**: 15-30 minutes
- **API cache**: 1-5 minutes
- **Configuration**: 1-24 hours
- **Temporary locks**: 10-60 seconds
- **Daily aggregates**: 24 hours

### 3. Data Size Considerations

- Keep individual values under 1MB
- Use arrays for lists under 1000 items
- Use objects for maps under 100 fields
- Consider data compression for large values

### 4. Error Handling

Always check return values:

```bash
# Check if SET succeeded
SET mykey myvalue
# Expected: OK

# Check if SETNX succeeded
SETNX lock:resource "owner"
# Returns: 1 (success) or 0 (already exists)

# Handle missing keys
GET nonexistent:key
# Returns: NOT_FOUND
```

### 5. Atomic Operations

Use NX commands for race-condition-free operations:

```bash
# Safe lock acquisition
SETNX lock:resource -t 30s "process_id"

# Safe initialization
SETSNX user:permissions -t 1h read write

# Safe configuration setup
SETXNX app:settings -t 24h theme light lang en
```

## Performance Tips

### 1. Batch Operations

```bash
# Efficient: Get multiple keys at once
GET user:name user:email user:age

# Less efficient: Multiple single-key gets
GET user:name
GET user:email
GET user:age
```

### 2. Appropriate TTL

- Don't use very short TTLs (< 1 second) unless necessary
- Don't use very long TTLs (> 1 day) for frequently changing data
- Use reasonable TTLs to balance freshness and performance

### 3. Connection Management

- Reuse connections when possible
- Use connection pooling in high-traffic applications
- Close connections properly

## Monitoring

### Key Statistics

```bash
# List all keys to see what's stored
KEYS

# Monitor key count and types
# (You can parse the KEYS output)
```

### Health Checks

```bash
# Simple health check
echo "KEYS" | nc localhost 8890

# Connection test
./ant-cache -cli
> SET health:check ok
> GET health:check
> exit
```

## Integration Examples

### Go Client

```go
conn, err := net.Dial("tcp", "localhost:8890")
if err != nil {
    log.Fatal(err)
}
defer conn.Close()

// Set value
fmt.Fprintf(conn, "SET mykey myvalue\n")

// Read response
scanner := bufio.NewScanner(conn)
scanner.Scan()
fmt.Println(scanner.Text()) // Should print "OK"
```

### Python Client

```python
import socket

sock = socket.socket(socket.AF_INET, socket.SOCK_STREAM)
sock.connect(('localhost', 8890))

# Set value
sock.send(b'SET mykey myvalue\n')
response = sock.recv(1024)
print(response.decode()) # Should print "OK"

sock.close()
```

## Troubleshooting

### Common Issues

**Commands not working:**
- Check if you're authenticated (if auth is enabled)
- Verify command syntax
- Ensure connection is established

**Data not persisting:**
- Check disk space
- Verify file permissions
- Check persistence configuration

**Performance issues:**
- Monitor memory usage
- Check TTL settings
- Review key naming patterns

### Getting Help

- Check the [Commands Reference](COMMANDS.md) for syntax
- Review [Installation Guide](INSTALLATION.md) for configuration
- See [Performance Report](PERFORMANCE.md) for optimization tips

## Next Steps

- Learn all available commands in [Commands Reference](COMMANDS.md)
- Optimize performance with [Performance Report](PERFORMANCE.md)
- Set up production deployment with [Installation Guide](INSTALLATION.md)
