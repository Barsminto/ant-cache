# Ant-Cache

A high-performance, lightweight in-memory cache system with Redis-like functionality.

## Overview

Ant-Cache is a fast, lightweight caching solution built in Go. It provides Redis-like commands with support for multiple data types, TTL expiration, data persistence, and optional authentication.

### Key Features

- **Multiple Data Types**: String, Array, Object
- **TTL Support**: Flexible expiration with time units (s/m/h/d/y)
- **Atomic Operations**: SETNX family commands for concurrency safety
- **Data Persistence**: ATD snapshots + ACL command logs
- **High Performance**: 1M+ operations per second
- **Simple Deployment**: Single binary, no dependencies

## Quick Start

```bash
# Start interactive CLI
./ant-cache -cli

# Basic usage
> SET user:1 "John Doe"
OK
> GET user:1
John Doe
> SET session -t 30m "session_data"
OK
```

## Installation

Download the latest release for your platform or build from source:

```bash
go build -o ant-cache
```

Requires a `config.json` file in the current directory. See [Installation Guide](docs/INSTALLATION.md) for details.

## Documentation

- **[Installation Guide](docs/INSTALLATION.md)** - Setup and configuration
- **[Usage Guide](docs/USAGE.md)** - How to use Ant-Cache
- **[Commands Reference](docs/COMMANDS.md)** - Complete command list
- **[Performance Report](docs/PERFORMANCE.md)** - Benchmarks and optimization

## Configuration

Create a `config.json` file:

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

## License

MIT License
