package main

import (
	"ant-cache/auth"
	"ant-cache/cache"
	"ant-cache/cleaner"
	"ant-cache/cli"
	"ant-cache/config"
	"ant-cache/tcpserver"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"
)

// handleQueryConfig handles the query configuration command
func handleQueryConfig(configFile string) {
	cfg, err := config.LoadConfig(configFile)
	if err != nil {
		log.Fatalf("Failed to load config file '%s': %v\nPlease ensure the config file exists and is valid JSON.", configFile, err)
	}

	fmt.Printf("=== Ant-Cache Configuration ===\n")
	fmt.Printf("Config File: %s\n", configFile)
	fmt.Printf("\n[Server]\n")
	fmt.Printf("Host: %s\n", cfg.Server.Host)
	fmt.Printf("Port: %s\n", cfg.Server.Port)

	fmt.Printf("\n[Persistence]\n")
	fmt.Printf("ATD Interval: %s\n", cfg.Persistence.AtdInterval)
	fmt.Printf("ACL Interval: %s\n", cfg.Persistence.AclInterval)

	fmt.Printf("\n[Authentication]\n")
	if cfg.Auth.Password != "" {
		fmt.Printf("Enabled: true\n")
		fmt.Printf("Password: %s\n", cfg.Auth.Password)
	} else {
		fmt.Printf("Enabled: false\n")
		fmt.Printf("Password: (not set)\n")
	}
}

func main() {
	// Parse command line arguments
	cliMode := flag.Bool("cli", false, "Run in interactive CLI mode")
	host := flag.String("host", "localhost", "Server host")
	port := flag.String("port", "8890", "Server port")
	configFile := flag.String("config", "", "Configuration file path (default: config.json in current directory)")
	atdFile := flag.String("atd", "", "ATD file path (empty to disable)")
	aclFile := flag.String("acl", "", "ACL file path (empty to disable)")
	atdInterval := flag.Duration("atd-interval", 1*time.Hour, "ATD save interval (min 5m, max 30d)")
	aclInterval := flag.Duration("acl-interval", 1*time.Second, "ACL sync interval (min 1s, max 1m)")

	queryConfig := flag.Bool("query", false, "Query current configuration")
	serverType := flag.String("server", "single-goroutine", "Server type: 'single-goroutine' or 'pooled-goroutine' (default: single-goroutine)")
	maxWorkers := flag.Int("workers", 200, "Number of worker goroutines for pooled server (default: 200)")
	flag.Parse()

	// Determine config file path
	var configPath string
	if *configFile != "" {
		// User specified config file
		configPath = *configFile
	} else {
		// Default to config.json in current directory
		configPath = "config.json"
	}

	// Handle query command
	if *queryConfig {
		handleQueryConfig(configPath)
		return
	}

	// Load configuration - required
	cfg, err := config.LoadConfig(configPath)
	if err != nil {
		if *configFile != "" {
			log.Fatalf("Failed to load specified config file '%s': %v\nPlease ensure the config file exists and is valid JSON.", configPath, err)
		} else {
			log.Fatalf("Failed to load default config file '%s': %v\nPlease ensure config.json exists in the current directory or specify a config file with -config flag.", configPath, err)
		}
	}
	log.Printf("Loaded configuration from: %s", configPath)

	// Setup authentication based on config
	var authManager *auth.AuthManager
	if cfg.Auth.Password != "" {
		// Authentication enabled if password is configured
		authManager = auth.NewAuthManager("auth.dat", true)
		if err := authManager.SetPassword(cfg.Auth.Password); err != nil {
			log.Printf("Failed to set password from config: %v", err)
		} else {
			log.Printf("Authentication enabled with password from config")
		}
	} else {
		// No authentication if no password configured
		authManager = auth.NewAuthManager("", false)
		log.Printf("Authentication disabled (no password configured)")
	}

	// Setup persistence with fixed paths
	atdPath := "cache.atd"
	aclPath := "cache.acl"

	// Parse intervals from config
	atdIntervalDuration, err := time.ParseDuration(cfg.Persistence.AtdInterval)
	if err != nil {
		log.Printf("Invalid ATD interval: %s, using default 1h", cfg.Persistence.AtdInterval)
		atdIntervalDuration = 1 * time.Hour
	}
	aclIntervalDuration, err := time.ParseDuration(cfg.Persistence.AclInterval)
	if err != nil {
		log.Printf("Invalid ACL interval: %s, using default 1s", cfg.Persistence.AclInterval)
		aclIntervalDuration = 1 * time.Second
	}

	// Override with command line arguments if provided
	if *atdFile != "" {
		atdPath = *atdFile
	}
	if *aclFile != "" {
		aclPath = *aclFile
	}
	if *atdInterval != 1*time.Hour {
		atdIntervalDuration = *atdInterval
	}
	if *aclInterval != 1*time.Second {
		aclIntervalDuration = *aclInterval
	}

	// Create cache with persistence (always enabled)
	log.Printf("Creating cache with persistence enabled")
	log.Printf("ATD file: %s, ACL file: %s", atdPath, aclPath)
	log.Printf("ATD interval: %v, ACL interval: %v", atdIntervalDuration, aclIntervalDuration)

	// Create compression config from configuration
	compressionConfig := cache.CompressionConfig{
		Enabled:     cfg.Compression.Enabled,
		MinSize:     cfg.Compression.MinSize,
		StringsOnly: cfg.Compression.StringsOnly,
	}

	if cfg.Compression.Enabled {
		log.Printf("Compression enabled: min_size=%d, strings_only=%v",
			cfg.Compression.MinSize, cfg.Compression.StringsOnly)
	} else {
		log.Printf("Compression disabled")
	}

	var cacheInstance *cache.Cache
	if authManager != nil {
		cacheInstance = cache.NewWithPersistenceAndAuth(atdPath, aclPath, atdIntervalDuration, aclIntervalDuration, authManager)
	} else {
		cacheInstance = cache.NewWithPersistence(atdPath, aclPath, atdIntervalDuration, aclIntervalDuration)
	}

	// Apply compression config
	cacheInstance.SetCompressionConfig(compressionConfig)

	// If configuration file loaded successfully, use config values
	if cfg != nil {
		*host = cfg.Server.Host
		*port = cfg.Server.Port

		// Start cleaner with default interval
		go cleaner.Start(cacheInstance)
	}
	// Setup graceful shutdown
	setupGracefulShutdown(cacheInstance)

	if *cliMode {
		cli.StartInteractiveCLI(cacheInstance, cfg.Server.Host, cfg.Server.Port, cfg.Auth.Password)
		// Call Close in CLI mode to save data
		cacheInstance.Close()
		os.Exit(0)
	}

	// Start TCP server
	log.Printf("Starting %s TCP cache server on %s:%s", *serverType, cfg.Server.Host, cfg.Server.Port)

	switch *serverType {
	case "single-goroutine":
		// Single-threaded listener with one goroutine per connection (direct cache memory access)
		fmt.Println("Starting single-goroutine server...")
		singleServer := tcpserver.NewSingleGoroutineServer(cacheInstance)
		if err := singleServer.Start(cfg.Server.Host, cfg.Server.Port); err != nil {
			log.Fatal(err)
		}

	case "pooled-goroutine":
		// Single-threaded listener with goroutine pool (direct cache memory access)
		fmt.Printf("Starting pooled-goroutine server (%d workers)...\n", *maxWorkers)
		pooledServer := tcpserver.NewPooledGoroutineServer(cacheInstance, *maxWorkers)
		if err := pooledServer.Start(cfg.Server.Host, cfg.Server.Port); err != nil {
			log.Fatal(err)
		}

	default:
		log.Fatalf("Unknown server type: %s. Available types: single-goroutine, pooled-goroutine", *serverType)
	}
}

func setupGracefulShutdown(cacheInstance *cache.Cache) {
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		<-sigChan
		log.Println("Received shutdown signal, saving data and shutting down...")
		cacheInstance.Close()
		os.Exit(0)
	}()
}
