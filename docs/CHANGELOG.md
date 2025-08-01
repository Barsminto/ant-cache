# Changelog

All notable changes to Ant-Cache will be documented in this file.

## [v1.1.0] - 2025-08-01

### 🚀 Architecture Optimization

#### Server Architecture Redesign
- **Single-Goroutine Server**: One goroutine per connection for maximum performance (104K+ ops/sec)
- **Pooled-Goroutine Server**: Fixed goroutine pool for resource control (103K+ ops/sec)
- **Removed Legacy**: Cleaned up old goroutine and multiplexed server implementations
- **Default Changed**: Now defaults to single-goroutine server for optimal performance

#### Performance Improvements
- **Direct Cache Access**: Eliminated command channel bottlenecks
- **Memory Optimization**: Reduced garbage collection pressure
- **Resource Control**: Pooled architecture provides predictable memory usage

### 🔧 Technical Improvements

#### Code Organization
- **Utils Package**: Shared command parsing utilities
- **Cleaner Architecture**: Simplified server implementations
- **Error Handling**: Improved error messages and validation

#### Documentation Overhaul
- **Complete Rewrite**: All documentation updated in English
- **Configuration Examples**: Ready-to-use config files provided
- **Performance Guide**: Detailed benchmarks and tuning recommendations

### 🛠️ Server Architecture Changes

#### Available Server Types
```bash
# Single-Goroutine (Default) - Maximum Performance
./ant-cache -server single-goroutine

# Pooled-Goroutine - Resource Controlled
./ant-cache -server pooled-goroutine -workers 200
```

#### Removed Server Types
- **goroutine**: Legacy implementation removed
- **multiplexed**: Legacy implementation removed

### 📊 Performance Results

| Architecture | Throughput | P95 Latency | Memory Control | Use Case |
|--------------|------------|-------------|----------------|----------|
| Single-Goroutine | 104,112 ops/sec | 1.89ms | Variable | High Performance |
| Pooled-Goroutine | 103,131 ops/sec | 1.91ms | Predictable | Production |

### 🔄 Migration Guide

#### Server Startup Changes
```bash
# Old (removed)
./ant-cache -server goroutine
./ant-cache -server multiplexed

# New (available)
./ant-cache                                         # Default: single-goroutine
./ant-cache -server single-goroutine               # Maximum performance
./ant-cache -server pooled-goroutine -workers 200  # Production recommended
```

### 📁 New Configuration Files

Ready-to-use configuration files added:
- `configs/production.json` - Production deployment
- `configs/development.json` - Development environment
- `configs/container.json` - Container deployment
- `configs/performance.json` - High-performance setup

### 📚 Documentation Structure

```
docs/
├── INSTALLATION.md    # Complete setup and deployment guide
├── COMMANDS.md       # All commands with examples
├── PERFORMANCE.md    # Benchmarks and optimization guide
└── CHANGELOG.md      # This file
```

### ⚠️ Breaking Changes

1. **Server Types**: Removed legacy `goroutine` and `multiplexed` server types
2. **Default Server**: Changed from `goroutine` to `single-goroutine`

**Note**: These are minor breaking changes that improve performance. Most existing usage will continue to work without modification.

### 🔧 Compatibility

- **Backward Compatible**: All existing commands work unchanged
- **Configuration**: Existing config files continue to work

---

## [v1.0.0] - Previous Version

### Features
- Basic SET, GET, DEL, KEYS, FLUSHALL commands
- TTL support for SET command
- Multiple server architectures
- Persistence with ATD and ACL files
- Authentication support

---

## Upgrade Instructions

### From v1.0 to v1.1

1. **Backup Data**: Ensure your cache data is backed up
2. **Update Binary**: Replace with new ant-cache binary
3. **Update Startup**: Change server type if using removed types (`goroutine` or `multiplexed`)
4. **Test Performance**: Verify performance improvements with new architectures

### Recommended Settings

```bash
# For maximum performance
./ant-cache -server single-goroutine

# For production stability
./ant-cache -server pooled-goroutine -workers 200 -config configs/production.json
```

### Support

- **Documentation**: See `/docs` directory for complete guides
- **Issues**: Report issues on GitHub
- **Migration Help**: Check migration examples in documentation
