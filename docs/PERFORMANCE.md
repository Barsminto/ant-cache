# Performance Guide

This guide provides comprehensive performance benchmarks, tuning recommendations, and scaling strategies for Ant-Cache.

## Benchmark Results

### Test Environment
- **Hardware**: Standard development machine
- **Test Load**: 100,000 SET operations
- **Concurrency**: 100 concurrent clients
- **Operation**: 1,000 operations per client
- **Network**: Local TCP connections

### Architecture Performance Comparison

| Architecture | Throughput | Avg Latency | P95 Latency | P99 Latency | Error Rate |
|--------------|------------|-------------|-------------|-------------|------------|
| **Single-Goroutine** | **104,112 ops/sec** | 945µs | **1.89ms** | 2.69ms | 0% |
| **Pooled-Goroutine** | **103,131 ops/sec** | 945µs | **1.91ms** | 2.69ms | 0% |

### Performance Analysis

**Single-Goroutine Server:**
- ✅ **Highest throughput**: 104,112 ops/sec
- ✅ **Lowest latency**: P95 = 1.89ms
- ⚠️ **Variable memory**: Scales with connection count
- ⚠️ **Resource spikes**: Can consume unlimited goroutines

**Pooled-Goroutine Server:**
- ⚡ **Excellent throughput**: 103,131 ops/sec (99.1% of single-goroutine)
- ⚡ **Consistent latency**: P95 = 1.91ms
- ✅ **Predictable memory**: Fixed resource usage
- ✅ **Graceful degradation**: Controlled resource consumption

## Architecture Selection Guide

### Single-Goroutine Server

**Choose when:**
- Maximum performance is critical
- Connection count is predictable and moderate (< 1000)
- Development or testing environment
- Variable load patterns
- Memory usage can fluctuate

**Configuration:**
```bash
# Maximum performance setup
./ant-cache -server single-goroutine

# With custom configuration
./ant-cache -server single-goroutine -config configs/performance.json
```

**Resource Characteristics:**
- **Memory**: 2KB per connection (goroutine stack)
- **Goroutines**: 1 per active connection
- **CPU**: Scales linearly with connections
- **Scalability**: Limited by system goroutine limits

### Pooled-Goroutine Server

**Choose when:**
- Production deployment
- Container or cloud environment
- High concurrency (1000+ connections)
- Predictable resource usage required
- Memory limits must be respected

**Configuration:**
```bash
# Production setup
./ant-cache -server pooled-goroutine -workers 200

# High-load setup
./ant-cache -server pooled-goroutine -workers 500

# Resource-constrained setup
./ant-cache -server pooled-goroutine -workers 100
```

**Resource Characteristics:**
- **Memory**: Fixed based on worker count
- **Goroutines**: Fixed pool size
- **CPU**: Controlled and predictable
- **Scalability**: Horizontal scaling friendly

## Pool Size Optimization

### Sizing Guidelines

| Load Profile | Ops/Sec | Connections | Recommended Workers | Memory Usage |
|--------------|---------|-------------|-------------------|--------------|
| **Light** | < 1,000 | < 100 | 25-50 | ~100KB |
| **Medium** | 1K-10K | 100-500 | 50-100 | ~200KB |
| **High** | 10K-50K | 500-2000 | 100-200 | ~400KB |
| **Extreme** | 50K+ | 2000+ | 200-500 | ~1MB |

### Pool Size Testing Results

Based on optimization testing with 50K operations and 50 concurrent clients:

| Workers | Throughput | Latency | Memory | Efficiency |
|---------|------------|---------|---------|------------|
| 25 | 85,234 ops/sec | 580µs | ~50KB | Under-utilized |
| 50 | 94,567 ops/sec | 525µs | ~100KB | Good |
| 100 | 101,245 ops/sec | 490µs | ~200KB | Very Good |
| **200** | **106,168 ops/sec** | **470µs** | **~400KB** | **Optimal** |
| 300 | 105,892 ops/sec | 475µs | ~600KB | Over-provisioned |
| 500 | 104,123 ops/sec | 485µs | ~1MB | Diminishing returns |

**Recommendation**: **200 workers** provides optimal balance of performance and resource usage.

### Dynamic Pool Sizing

The pooled-goroutine server includes intelligent scaling:

```go
// Automatic scaling triggers
Scale Up When:
- Queue length > 50% of worker count
- 90%+ workers are active
- Average task time is increasing

Scale Down When:
- Queue is empty
- Average task time < 1ms
- Low worker utilization
```

## Performance Tuning

### System-Level Optimizations

**File Descriptor Limits:**
```bash
# Check current limits
ulimit -n

# Increase for current session
ulimit -n 65536

# Permanent increase (add to /etc/security/limits.conf)
echo "* soft nofile 65536" >> /etc/security/limits.conf
echo "* hard nofile 65536" >> /etc/security/limits.conf
```

**TCP Tuning:**
```bash
# Optimize TCP settings
echo 'net.core.somaxconn = 65535' >> /etc/sysctl.conf
echo 'net.ipv4.tcp_max_syn_backlog = 65535' >> /etc/sysctl.conf
echo 'net.core.netdev_max_backlog = 5000' >> /etc/sysctl.conf
echo 'net.ipv4.tcp_fin_timeout = 30' >> /etc/sysctl.conf

# Apply changes
sysctl -p
```

**Memory Optimization:**
```bash
# Reduce swap usage
echo 'vm.swappiness = 1' >> /etc/sysctl.conf

# Optimize memory allocation
echo 'vm.overcommit_memory = 1' >> /etc/sysctl.conf
```

### Application-Level Optimizations

**Connection Pooling (Client Side):**
```go
// Example Go client with connection pooling
type ConnectionPool struct {
    connections chan net.Conn
    maxSize     int
}

func (p *ConnectionPool) Get() net.Conn {
    select {
    case conn := <-p.connections:
        return conn
    default:
        conn, _ := net.Dial("tcp", "localhost:8890")
        return conn
    }
}

func (p *ConnectionPool) Put(conn net.Conn) {
    select {
    case p.connections <- conn:
    default:
        conn.Close()
    }
}
```

**Batch Operations:**
```bash
# Instead of individual operations
SET key1 value1
SET key2 value2
SET key3 value3

# Use pipelining or connection reuse
# (Keep connection open for multiple operations)
```

## Scaling Strategies

### Vertical Scaling

**Hardware Upgrades:**
- **CPU**: More cores improve concurrent processing
- **Memory**: More RAM allows larger datasets and more connections
- **Network**: Faster network reduces latency
- **Storage**: SSD improves persistence performance

**Configuration Adjustments:**
```bash
# Scale up workers for more CPU cores
./ant-cache -server pooled-goroutine -workers 400  # For 8+ core systems

# Increase system limits
ulimit -n 100000  # More file descriptors
```

### Horizontal Scaling

**Load Balancing:**
```bash
# Multiple Ant-Cache instances
./ant-cache -port 8890 -server pooled-goroutine -workers 200
./ant-cache -port 8891 -server pooled-goroutine -workers 200
./ant-cache -port 8892 -server pooled-goroutine -workers 200

# Use load balancer (nginx, HAProxy, etc.)
```

**Sharding Strategy:**
```python
# Client-side sharding example
def get_server_port(key):
    hash_value = hash(key) % 3
    return 8890 + hash_value

def set_value(key, value):
    port = get_server_port(key)
    # Connect to appropriate server
    conn = connect(f"localhost:{port}")
    conn.send(f"SET {key} {value}\n")
```

### Manual Scaling

**Multiple Instance Setup:**
```bash
# Start multiple instances on different ports
./ant-cache -port 8890 -server pooled-goroutine -workers 200 &
./ant-cache -port 8891 -server pooled-goroutine -workers 200 &
./ant-cache -port 8892 -server pooled-goroutine -workers 200 &

# Each instance runs independently
# Use process management tools like systemd for production
```

**Process Management with systemd:**
```bash
# Create multiple service files
sudo cp /etc/systemd/system/ant-cache.service /etc/systemd/system/ant-cache-1.service
sudo cp /etc/systemd/system/ant-cache.service /etc/systemd/system/ant-cache-2.service
sudo cp /etc/systemd/system/ant-cache.service /etc/systemd/system/ant-cache-3.service

# Edit each service file to use different ports
# ant-cache-1.service: -port 8890
# ant-cache-2.service: -port 8891
# ant-cache-3.service: -port 8892

# Start all instances
sudo systemctl start ant-cache-1 ant-cache-2 ant-cache-3
```

## Monitoring and Metrics

### Performance Monitoring

**Built-in Benchmark:**
```bash
# Run performance benchmark
./scripts/performance_benchmark.sh

# Results include:
# - Throughput (ops/sec)
# - Latency percentiles (P50, P95, P99)
# - Error rates
# - Resource usage
```

**System Monitoring:**
```bash
# Monitor CPU and memory
top -p $(pgrep ant-cache)

# Monitor network connections
netstat -an | grep :8890 | wc -l

# Monitor file descriptors
lsof -p $(pgrep ant-cache) | wc -l
```

### Key Performance Indicators

**Throughput Metrics:**
- Operations per second (target: 100K+)
- Concurrent connections (monitor growth)
- Queue depth (pooled-goroutine only)

**Latency Metrics:**
- Average response time (target: < 1ms)
- P95 latency (target: < 2ms)
- P99 latency (target: < 5ms)

**Resource Metrics:**
- Memory usage (should be predictable)
- CPU utilization (should scale with load)
- Network bandwidth utilization

**Error Metrics:**
- Connection errors (should be 0%)
- Timeout errors (should be < 0.1%)
- Command errors (application dependent)

## Troubleshooting Performance Issues

### High Latency

**Symptoms:**
- P95 latency > 5ms
- Slow response times

**Solutions:**
```bash
# Check system load
top
iostat 1

# Reduce worker count if over-provisioned
./ant-cache -server pooled-goroutine -workers 100

# Check network issues
ping localhost
netstat -i
```

### Low Throughput

**Symptoms:**
- Ops/sec significantly below benchmarks
- High CPU usage with low throughput

**Solutions:**
```bash
# Increase worker count
./ant-cache -server pooled-goroutine -workers 300

# Check for resource constraints
free -h
df -h

# Switch to single-goroutine for maximum performance
./ant-cache -server single-goroutine
```

### Memory Issues

**Symptoms:**
- High memory usage
- Out of memory errors

**Solutions:**
```bash
# Use pooled-goroutine with controlled workers
./ant-cache -server pooled-goroutine -workers 100

# Monitor memory usage
ps aux | grep ant-cache

# Check for memory leaks
valgrind ./ant-cache
```

## Extreme Performance Targets

### Reaching 500K+ ops/sec

**Requirements:**
- **Hardware**: 16+ CPU cores, 32GB+ RAM, 10Gbps+ network
- **System Tuning**: Optimized kernel parameters
- **Application**: Pooled-goroutine with 500+ workers

**Configuration:**
```bash
# High-performance setup
./ant-cache -server pooled-goroutine -workers 500 -config configs/extreme-performance.json

# System tuning required
echo 'net.core.somaxconn = 65535' >> /etc/sysctl.conf
echo 'fs.file-max = 1000000' >> /etc/sysctl.conf
ulimit -n 1000000
```

### Theoretical Limits

**Single Machine Limits:**
- **CPU Bound**: ~1M ops/sec per core for simple operations
- **Memory Bound**: Limited by available RAM
- **Network Bound**: Limited by network bandwidth and packet rate
- **System Bound**: File descriptor and connection limits

**Practical Limits:**
- **Single-Goroutine**: ~150K ops/sec (with optimal hardware)
- **Pooled-Goroutine**: ~200K ops/sec (with 1000+ workers)
- **Horizontal Scaling**: Unlimited (with proper sharding)

## Next Steps

- Review [Installation Guide](INSTALLATION.md) for deployment options
- Check [Commands Reference](COMMANDS.md) for usage patterns
- Monitor your deployment and adjust based on actual load patterns
- Consider horizontal scaling for loads exceeding single-machine limits
