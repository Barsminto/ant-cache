# Installation Guide

This guide covers how to install, configure, and deploy Ant-Cache.

## System Requirements

- **Operating System**: Linux, macOS, Windows
- **Memory**: 512MB minimum, 2GB+ recommended
- **Disk**: 100MB for program and data files
- **Go**: 1.19+ (if building from source)

## Installation Methods

### Method 1: Download Release

1. Download the latest release for your platform from GitHub Releases
2. Extract the archive:
   ```bash
   tar -xzf ant-cache-v1.0.0-linux-amd64.tar.gz
   cd ant-cache-v1.0.0-linux-amd64
   ```
3. Run the installer (Linux/macOS):
   ```bash
   sudo ./scripts/install.sh
   ```

### Method 2: Build from Source

```bash
# Clone repository
git clone <repository-url>
cd ant-cache

# Build
go build -o ant-cache

# Create config file
cp configs/config.json .
```

## Configuration

### Required Configuration File

Ant-Cache requires a `config.json` file in the current directory or specified with `-config` flag.

#### Default Configuration

```json
{
  "server": {
    "host": "localhost",
    "port": "8890"
  },
  "auth": {
    "password": ""
  },
  "persistence": {
    "atd_interval": "1h",
    "acl_interval": "1s"
  }
}
```

#### Configuration Options

**Server Settings:**
- `host`: Server listening address (default: "localhost")
- `port`: Server listening port (default: "8890")

**Authentication:**
- `password`: Authentication password (empty = no auth)

**Persistence:**
- `atd_interval`: Snapshot save interval (e.g., "30m", "1h", "2h")
- `acl_interval`: Command log sync interval (e.g., "1s", "5s")

### Production Configuration

For production environments, create a secure configuration:

```json
{
  "server": {
    "host": "0.0.0.0",
    "port": "8890"
  },
  "auth": {
    "password": "your_secure_password_here"
  },
  "persistence": {
    "atd_interval": "30m",
    "acl_interval": "1s"
  }
}
```

## Running Ant-Cache

### CLI Mode (Interactive)

```bash
# Use default config.json
./ant-cache -cli

# Use specific config file
./ant-cache -config /path/to/config.json -cli
```

If authentication is enabled, you'll be prompted for a password.

### Server Mode

```bash
# Start TCP server
./ant-cache

# Use specific config
./ant-cache -config /path/to/config.json
```

### Command Line Options

```bash
./ant-cache [options]

Options:
  -cli                Start interactive CLI mode
  -config string      Configuration file path (default: config.json)
  -query             Show current configuration
  -h                 Show help
```

## System Service Setup

### Linux (systemd)

Create `/etc/systemd/system/ant-cache.service`:

```ini
[Unit]
Description=Ant-Cache Memory Cache Server
After=network.target

[Service]
Type=simple
User=ant-cache
Group=ant-cache
WorkingDirectory=/opt/ant-cache
ExecStart=/opt/ant-cache/ant-cache -config /opt/ant-cache/config.json
Restart=always
RestartSec=5

[Install]
WantedBy=multi-user.target
```

Enable and start:

```bash
sudo systemctl daemon-reload
sudo systemctl enable ant-cache
sudo systemctl start ant-cache
```

### Docker

Create `Dockerfile`:

```dockerfile
FROM golang:1.21-alpine AS builder
WORKDIR /app
COPY . .
RUN go build -o ant-cache

FROM alpine:latest
RUN apk --no-cache add ca-certificates
WORKDIR /root/
COPY --from=builder /app/ant-cache .
COPY --from=builder /app/config.json .
EXPOSE 8890
CMD ["./ant-cache"]
```

Build and run:

```bash
docker build -t ant-cache .
docker run -d -p 8890:8890 ant-cache
```

## Data Persistence

### File Locations

- **ATD File**: `cache.atd` (binary snapshot)
- **ACL File**: `cache.acl` (command log)
- **Auth File**: `auth.dat` (encrypted passwords)

### Backup Strategy

```bash
# Backup data files
cp cache.atd cache.atd.backup
cp cache.acl cache.acl.backup
cp config.json config.json.backup
```

### Recovery

```bash
# Restore from backup
cp cache.atd.backup cache.atd
cp cache.acl.backup cache.acl
./ant-cache
```

## Security

### Authentication

Enable authentication by setting a password in config.json:

```json
{
  "auth": {
    "password": "your_secure_password"
  }
}
```

### Network Security

- Bind to localhost for local-only access
- Use firewall rules for remote access
- Consider TLS proxy for encrypted connections

### File Permissions

```bash
chmod 600 config.json    # Config file
chmod 600 auth.dat       # Password file
chmod 644 cache.atd      # Data files
chmod 644 cache.acl
```

## Troubleshooting

### Common Issues

**Config file not found:**
```bash
# Ensure config.json exists in current directory
ls -la config.json

# Or specify config file path
./ant-cache -config /path/to/config.json
```

**Port already in use:**
```bash
# Check what's using the port
lsof -i :8890

# Use different port in config.json
```

**Permission denied:**
```bash
# Check file permissions
ls -la cache.*

# Fix permissions
chmod 644 cache.atd cache.acl
```

### Verification

```bash
# Check configuration
./ant-cache -query

# Test connection
echo "KEYS" | nc localhost 8890

# Check logs
journalctl -u ant-cache -f
```

## Performance Tuning

### Memory Settings

```bash
export GOGC=100          # Adjust GC frequency
export GOMEMLIMIT=2GiB   # Limit memory usage
```

### File Descriptors

```bash
ulimit -n 65536          # Increase file descriptor limit
```

### Persistence Tuning

- **High write load**: Decrease `acl_interval` to "100ms"
- **Low write load**: Increase `atd_interval` to "2h"
- **Memory constrained**: Increase `atd_interval`, decrease `acl_interval`

## Next Steps

- Read the [Usage Guide](USAGE.md) to learn how to use Ant-Cache
- Check the [Commands Reference](COMMANDS.md) for all available commands
- Review [Performance Report](PERFORMANCE.md) for optimization tips
