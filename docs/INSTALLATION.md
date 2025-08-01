# Installation & Usage Guide

This guide covers installation, configuration, and deployment of Ant-Cache.

## System Requirements

- **Go**: 1.19 or later
- **OS**: Linux, macOS, Windows
- **Memory**: 512MB minimum, 2GB+ recommended for production
- **CPU**: 2+ cores recommended
- **Network**: TCP port access (default: 8890)

## Installation

### From Source

```bash
# Clone the repository
git clone https://github.com/yourusername/ant-cache.git
cd ant-cache

# Build the binary
go build -o ant-cache

# Verify installation
./ant-cache -h
```

### Using Build Script

```bash
# Build optimized release binary
./scripts/build-release.sh

# Binary will be created as ant-cache
```

## Basic Usage

### Starting the Server

```bash
# Default mode (single-goroutine, maximum performance)
./ant-cache

# Pooled-goroutine mode (production recommended)
./ant-cache -server pooled-goroutine -workers 200

# With custom host and port
./ant-cache -host 0.0.0.0 -port 9000

# With configuration file
./ant-cache -config configs/production.json

# Background mode
nohup ./ant-cache -config configs/production.json > ant-cache.log 2>&1 &
```

### Command Line Options

```bash
Usage: ./ant-cache [options]

Options:
  -server string
        Server architecture: 'single-goroutine' or 'pooled-goroutine' (default: single-goroutine)
  -workers int
        Number of worker goroutines for pooled server (default: 200)
  -host string
        Server host address (default: localhost)
  -port string
        Server port (default: 8890)
  -config string
        Configuration file path
  -cli
        Start in interactive CLI mode
  -h, -help
        Show help message
```

### Interactive CLI Mode

```bash
# Start CLI mode
./ant-cache -cli

# CLI session example
> SET user:1001 "John Doe"
OK
> GET user:1001
John Doe
> SET session:abc123 -t 3600s "active"
OK
> KEYS user:*
user:1001
> FLUSHALL
OK
> exit
```

## Server Architectures

### Single-Goroutine Server

**Best for**: Maximum performance, development, variable load patterns

```bash
# Start single-goroutine server
./ant-cache -server single-goroutine

# Performance characteristics
# - Throughput: ~104K ops/sec
# - Memory: Dynamic (scales with connections)
# - Goroutines: One per connection
# - Resource usage: Variable
```

**Use Cases**:
- High-performance applications
- Development and testing
- Applications with variable connection patterns
- When maximum throughput is required

### Pooled-Goroutine Server

**Best for**: Production environments, resource control, containers

```bash
# Start pooled-goroutine server
./ant-cache -server pooled-goroutine -workers 200

# Performance characteristics
# - Throughput: ~103K ops/sec (99.1% of single-goroutine)
# - Memory: Fixed and predictable
# - Goroutines: Fixed pool size
# - Resource usage: Controlled
```

**Use Cases**:
- Production deployments
- Container environments with memory limits
- High-concurrency scenarios (1000+ connections)
- Enterprise applications requiring stability

### Pool Size Configuration

| Scenario | Workers | Expected Load | Memory Usage |
|----------|---------|---------------|--------------|
| **Development** | 25-50 | < 1K ops/sec | ~100KB |
| **Small Production** | 50-100 | 1K-10K ops/sec | ~200KB |
| **Medium Production** | 100-200 | 10K-50K ops/sec | ~400KB |
| **High Load** | 200-500 | 50K+ ops/sec | ~1MB |

**Recommendation**: Start with **200 workers** and adjust based on monitoring.

## Configuration

### Configuration File Format

Ant-Cache uses JSON configuration files:

```json
{
  "server": {
    "host": "localhost",
    "port": "8890"
  },
  "persistence": {
    "atd_file": "cache.atd",
    "atd_interval": "1h",
    "acl_file": "cache.acl",
    "acl_interval": "1s"
  },
  "auth": {
    "password": ""
  }
}
```

### Configuration Options

#### Server Section
- `host`: Server bind address (default: "localhost")
- `port`: Server port (default: "8890")

#### Persistence Section
- `atd_file`: Snapshot file path (default: "cache.atd")
- `atd_interval`: Snapshot interval (default: "1h")
- `acl_file`: Append-only log file path (default: "cache.acl")
- `acl_interval`: Log flush interval (default: "1s")

#### Auth Section
- `password`: Authentication password (empty = no auth)

### Pre-configured Files

Use the provided configuration files in the `configs/` directory:

```bash
# Production deployment
./ant-cache -config configs/production.json

# Development environment
./ant-cache -config configs/development.json

# Container deployment
./ant-cache -config configs/container.json
```

## Deployment

### Production Deployment

```bash
# Create dedicated user
sudo useradd -r -s /bin/false ant-cache

# Create directories
sudo mkdir -p /opt/ant-cache /var/lib/ant-cache /var/log/ant-cache
sudo chown ant-cache:ant-cache /var/lib/ant-cache /var/log/ant-cache

# Copy binary and config
sudo cp ant-cache /opt/ant-cache/
sudo cp configs/production.json /opt/ant-cache/config.json

# Start service
sudo -u ant-cache /opt/ant-cache/ant-cache -config /opt/ant-cache/config.json
```

### Manual Multi-Instance Deployment

```bash
# Create separate directories for each instance
mkdir -p /opt/ant-cache-{1,2,3}

# Copy binary and configs
cp ant-cache /opt/ant-cache-1/
cp ant-cache /opt/ant-cache-2/
cp ant-cache /opt/ant-cache-3/

cp configs/production.json /opt/ant-cache-1/config.json
cp configs/production.json /opt/ant-cache-2/config.json
cp configs/production.json /opt/ant-cache-3/config.json

# Edit each config to use different ports
sed -i 's/"8890"/"8890"/' /opt/ant-cache-1/config.json
sed -i 's/"8890"/"8891"/' /opt/ant-cache-2/config.json
sed -i 's/"8890"/"8892"/' /opt/ant-cache-3/config.json

# Start instances
cd /opt/ant-cache-1 && ./ant-cache -config config.json &
cd /opt/ant-cache-2 && ./ant-cache -config config.json &
cd /opt/ant-cache-3 && ./ant-cache -config config.json &
```

### Systemd Service

Create `/etc/systemd/system/ant-cache.service`:

```ini
[Unit]
Description=Ant-Cache Server
After=network.target

[Service]
Type=simple
User=ant-cache
WorkingDirectory=/opt/ant-cache
ExecStart=/opt/ant-cache/ant-cache -config /opt/ant-cache/config.json
Restart=always
RestartSec=5

[Install]
WantedBy=multi-user.target
```

Enable and start:

```bash
sudo systemctl enable ant-cache
sudo systemctl start ant-cache
sudo systemctl status ant-cache
```

## Monitoring

### Health Check

```bash
# Simple connectivity test
echo "SET health_check ok" | nc localhost 8890

# Response should be: OK
```

### Performance Monitoring

```bash
# Run benchmark
./scripts/performance_benchmark.sh

# Monitor with system tools
top -p $(pgrep ant-cache)
netstat -an | grep :8890
```

### Log Monitoring

```bash
# Monitor server logs
tail -f ant-cache.log

# Monitor system logs
journalctl -u ant-cache -f
```

## Troubleshooting

### Common Issues

**Port already in use**:
```bash
# Check what's using the port
lsof -i :8890

# Use different port
./ant-cache -port 8891
```

**Permission denied**:
```bash
# Check file permissions
ls -la cache.atd cache.acl

# Fix permissions
chmod 644 cache.atd cache.acl
```

**High memory usage**:
```bash
# Switch to pooled-goroutine mode
./ant-cache -server pooled-goroutine -workers 100

# Monitor memory usage
ps aux | grep ant-cache
```

**Connection refused**:
```bash
# Check if server is running
ps aux | grep ant-cache

# Check network binding
netstat -tlnp | grep ant-cache

# Test local connection
telnet localhost 8890
```

### Performance Tuning

**System limits**:
```bash
# Increase file descriptor limit
ulimit -n 65536

# For permanent change, edit /etc/security/limits.conf
echo "* soft nofile 65536" >> /etc/security/limits.conf
echo "* hard nofile 65536" >> /etc/security/limits.conf
```

**Network tuning**:
```bash
# Optimize TCP settings
echo 'net.core.somaxconn = 65535' >> /etc/sysctl.conf
echo 'net.ipv4.tcp_max_syn_backlog = 65535' >> /etc/sysctl.conf
sysctl -p
```

## Next Steps

- Review [Commands Reference](COMMANDS.md) for available operations
- Check [Performance Guide](PERFORMANCE.md) for optimization tips
- Configure monitoring and alerting for production use
