# Ant-Cache

A high-performance, lightweight in-memory cache server written in Go with Redis-like commands and optimized architectures.

## Overview

Ant-Cache is designed for applications requiring fast, reliable caching with minimal overhead. It offers two optimized server architectures to balance performance and resource control based on your deployment needs.

**Key Features:**
- **100K+ ops/sec** performance with optimized architectures
- **Redis-compatible commands** for easy integration
- **Multiple data types**: strings, arrays, and objects
- **Flexible TTL support** with multiple time units
- **Persistent storage** with ATD snapshots and ACL logs
- **Two server architectures** for different use cases
- **Production-ready** with graceful shutdown and error handling

## Quick Start

### Installation

```bash
# Clone and build
git clone https://github.com/Barsminto/ant-cache.git
cd ant-cache
go build -o ant-cache

# Start with default settings
./ant-cache

# Connect and test different data types
echo "SET hello world" | nc localhost 8890
echo "SETS users alice bob charlie" | nc localhost 8890
echo "SETX config debug true port 8890" | nc localhost 8890
echo "GET hello" | nc localhost 8890
```

### Basic Usage

```bash
# Single-goroutine mode (default, maximum performance)
./ant-cache

# Pooled-goroutine mode (production, resource controlled)
./ant-cache -server pooled-goroutine -workers 200

# With custom configuration
./ant-cache -config configs/production.json

# Interactive CLI mode
./ant-cache -cli
```

## Server Architectures

### Single-Goroutine Server (Default)
- **Performance**: 104,112 ops/sec
- **Memory**: Dynamic (scales with connections)
- **Best for**: Maximum performance, development, variable load

### Pooled-Goroutine Server  
- **Performance**: 103,131 ops/sec (99.1% of single-goroutine)
- **Memory**: Fixed and predictable
- **Best for**: Production, containers, resource control

## Performance Results

Based on benchmark testing (100K operations, 100 concurrent clients):

| Architecture | Throughput | P95 Latency | Memory Control | Use Case |
|--------------|------------|-------------|----------------|----------|
| **Single-Goroutine** | **104,112 ops/sec** | 1.89ms | Variable | High Performance |
| **Pooled-Goroutine** | **103,131 ops/sec** | 1.91ms | Predictable | Production |

### Pool Size Recommendations

| Load Level | Operations/sec | Recommended Workers |
|------------|----------------|-------------------|
| Light | < 1,000 | 25-50 |
| Medium | 1K-10K | 50-100 |
| High | 10K-50K | 100-200 |
| Extreme | 50K+ | 200-500 |

**Default recommendation**: Start with **200 workers** for optimal balance.

## Documentation

- **[Installation & Usage Guide](docs/INSTALLATION.md)** - Complete setup and deployment instructions
- **[Commands Reference](docs/COMMANDS.md)** - All available commands with examples
- **[Performance Guide](docs/PERFORMANCE.md)** - Benchmarks, tuning, and scaling recommendations
- **[Changelog](docs/CHANGELOG.md)** - Version history and upgrade instructions

## Configuration Files

Ready-to-use configuration files are provided:

- `configs/production.json` - Production deployment settings
- `configs/development.json` - Development environment settings  
- `configs/container.json` - Container/cloud deployment settings

## License

MIT License - see [LICENSE](LICENSE) file for details.
