package cache

import (
	"bufio"
	"compress/gzip"
	"container/heap"
	"encoding/binary"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"
)

// PersistenceManager persistence manager
type PersistenceManager struct {
	cache          *Cache
	atdPath        string
	aclPath        string
	atdInterval    time.Duration
	aclInterval    time.Duration
	lastAtdTime    time.Time
	lastAclTime    time.Time
	mutex          sync.RWMutex
	enabled        bool
	stopChan       chan struct{}
	commandChan    chan Command
	aclFileSize    int64
	maxAclFileSize int64
}

// Command command struct
type Command struct {
	Timestamp int64
	Type      string
	Key       string
	Value     interface{}
	TTL       time.Duration
}

// Binary format constants for ATD
const (
	MAGIC_HEADER = uint32(0x414E5443) // "ANTC" (Ant Cache)
	VERSION      = 0x01
)

// Record types for ATD
const (
	RECORD_ITEM  = 0x01
	RECORD_STATS = 0x02
	RECORD_END   = 0xFF
)

// Command types
const (
	CMD_SET  = "SET"
	CMD_SETS = "SETS"
	CMD_SETX = "SETX"
	CMD_DEL  = "DEL"
	CMD_DELS = "DELS"
	CMD_DELX = "DELX"
)

// NewPersistenceManager create persistence manager
func NewPersistenceManager(cache *Cache, atdPath, aclPath string, atdInterval, aclInterval time.Duration) *PersistenceManager {
	return &PersistenceManager{
		cache:          cache,
		atdPath:        atdPath,
		aclPath:        aclPath,
		atdInterval:    atdInterval,
		aclInterval:    aclInterval,
		enabled:        true,
		stopChan:       make(chan struct{}),
		commandChan:    make(chan Command, 10000), // Larger buffer
		maxAclFileSize: 10 * 1024 * 1024,          // 10MB
	}
}

// Start start persistence manager
func (pm *PersistenceManager) Start() {
	if !pm.enabled {
		return
	}

	// Start ATD periodic save goroutine
	go pm.periodicAtd()
	// Start ACL periodic sync goroutine
	go pm.periodicAcl()
	// Start command processing goroutine
	go pm.processCommands()
}

// Stop stop persistence manager
func (pm *PersistenceManager) Stop() {
	if !pm.enabled {
		return
	}

	close(pm.stopChan)

	// wait for all commands to be processed
	time.Sleep(500 * time.Millisecond)

	close(pm.commandChan)

	// save one last time
	pm.SaveAtd()
	pm.flushAcl()
}

// LogCommand record command
func (pm *PersistenceManager) LogCommand(cmdType, key string, value interface{}, ttl time.Duration) {
	if !pm.enabled {
		return
	}

	select {
	case pm.commandChan <- Command{
		Timestamp: time.Now().UnixNano(),
		Type:      cmdType,
		Key:       key,
		Value:     value,
		TTL:       ttl,
	}:
	default:
		// Channel is full, drop command
		fmt.Printf("Warning: Command channel full, dropping command: %s %s\n", cmdType, key)
	}
}

// processCommands process commands
func (pm *PersistenceManager) processCommands() {
	for {
		select {
		case cmd, ok := <-pm.commandChan:
			if !ok {
				return
			}
			pm.writeCommandToAcl(cmd)
		case <-pm.stopChan:
			return
		}
	}
}

// writeCommandToAcl write command to acl
func (pm *PersistenceManager) writeCommandToAcl(cmd Command) {
	pm.mutex.Lock()
	defer pm.mutex.Unlock()

	dir := filepath.Dir(pm.aclPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		fmt.Printf("Failed to create directory: %v\n", err)
		return
	}

	if pm.aclFileSize >= pm.maxAclFileSize {
		pm.rotateAclFile()
	}

	file, err := os.OpenFile(pm.aclPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		fmt.Printf("Failed to open ACL file: %v\n", err)
		return
	}
	defer file.Close()

	// Format command
	var valueStr string
	switch v := cmd.Value.(type) {
	case []string:
		valueStr = fmt.Sprintf("[%s]", strings.Join(v, " "))
	case map[string]string:
		var pairs []string
		for k, v := range v {
			pairs = append(pairs, fmt.Sprintf("%s:%s", k, v))
		}
		valueStr = fmt.Sprintf("map[%s]", strings.Join(pairs, " "))
	default:
		valueStr = fmt.Sprintf("%v", v)
	}

	line := fmt.Sprintf("%d|%s|%s|%s|%d\n",
		cmd.Timestamp, cmd.Type, cmd.Key, valueStr, cmd.TTL.Nanoseconds())

	if _, err := file.WriteString(line); err != nil {
		fmt.Printf("Failed to write ACL: %v\n", err)
		return
	}

	pm.aclFileSize += int64(len(line))
}

// rotateAclFile rotate acl file
func (pm *PersistenceManager) rotateAclFile() {
	timestamp := time.Now().Format("20060102_150405")

	// rename old file
	oldPath := pm.aclPath
	newPath := fmt.Sprintf("%s.%s", pm.aclPath, timestamp)

	if err := os.Rename(oldPath, newPath); err != nil {
		fmt.Printf("Failed to rotate ACL file: %v\n", err)
		return
	}

	// reset file size
	pm.aclFileSize = 0

	fmt.Printf("ACL file rotated: %s -> %s\n", oldPath, newPath)
}

// flushAcl flush acl
func (pm *PersistenceManager) flushAcl() {
	// wait for all commands to be processed
	time.Sleep(100 * time.Millisecond)
	pm.compactAclFiles()
}

// compactAclFiles merge acl files
func (pm *PersistenceManager) compactAclFiles() {
	// find all acl files
	dir := filepath.Dir(pm.aclPath)
	pattern := filepath.Join(dir, filepath.Base(pm.aclPath)+".*")

	matches, err := filepath.Glob(pattern)
	if err != nil {
		fmt.Printf("Failed to find ACL files: %v\n", err)
		return
	}

	// also include the current file
	allFiles := append([]string{pm.aclPath}, matches...)

	if len(allFiles) <= 1 {
		return // Only one file, no need to merge
	}

	fmt.Printf("Found %d ACL files to compact\n", len(allFiles))

	// Read all commands
	commands := make(map[string]Command) // key -> latest command

	for _, filePath := range allFiles {
		file, err := os.Open(filePath)
		if err != nil {
			fmt.Printf("Failed to open ACL file %s: %v\n", filePath, err)
			continue
		}

		scanner := bufio.NewScanner(file)
		for scanner.Scan() {
			line := strings.TrimSpace(scanner.Text())
			if line == "" {
				continue
			}

			// Parse command line
			parts := strings.Split(line, "|")
			if len(parts) != 5 {
				continue
			}

			timestamp, _ := strconv.ParseInt(parts[0], 10, 64)
			cmdType := parts[1]
			key := parts[2]
			valueStr := parts[3]
			ttlNanos, _ := strconv.ParseInt(parts[4], 10, 64)

			// Parse value
			var value interface{}
			if strings.HasPrefix(valueStr, "[") && strings.HasSuffix(valueStr, "]") {
				content := strings.Trim(valueStr, "[]")
				if content != "" {
					value = strings.Fields(content)
				} else {
					value = []string{}
				}
			} else if strings.HasPrefix(valueStr, "map[") && strings.Contains(valueStr, ":") {
				content := strings.TrimPrefix(valueStr, "map[")
				content = strings.TrimSuffix(content, "]")
				pairs := strings.Fields(content)
				obj := make(map[string]string)
				for _, pair := range pairs {
					if strings.Contains(pair, ":") {
						kv := strings.SplitN(pair, ":", 2)
						if len(kv) == 2 {
							obj[kv[0]] = kv[1]
						}
					}
				}
				value = obj
			} else {
				value = valueStr
			}

			cmd := Command{
				Timestamp: timestamp,
				Type:      cmdType,
				Key:       key,
				Value:     value,
				TTL:       time.Duration(ttlNanos),
			}

			// Merge strategy: DELETE commands override SET commands, otherwise keep the latest
			if existingCmd, exists := commands[key]; exists {
				if cmdType == CMD_DEL || cmdType == CMD_DELS || cmdType == CMD_DELX {
					// DELETE commands always override SET commands
					commands[key] = cmd
				} else if existingCmd.Type == CMD_DEL || existingCmd.Type == CMD_DELS || existingCmd.Type == CMD_DELX {
					// If there's already a DELETE command, SET commands are ignored
					continue
				} else if cmd.Timestamp > existingCmd.Timestamp {
					// For commands of the same type, keep the latest
					commands[key] = cmd
				}
			} else {
				commands[key] = cmd
			}
		}

		file.Close()
	}

	// Create merged file
	mergedPath := pm.aclPath + ".merged"
	mergedFile, err := os.Create(mergedPath)
	if err != nil {
		fmt.Printf("Failed to create merged file: %v\n", err)
		return
	}
	defer mergedFile.Close()

	// Write merged commands
	for _, cmd := range commands {
		// 格式化命令
		var valueStr string
		switch v := cmd.Value.(type) {
		case []string:
			valueStr = fmt.Sprintf("[%s]", strings.Join(v, " "))
		case map[string]string:
			var pairs []string
			for k, v := range v {
				pairs = append(pairs, fmt.Sprintf("%s:%s", k, v))
			}
			valueStr = fmt.Sprintf("map[%s]", strings.Join(pairs, " "))
		default:
			valueStr = fmt.Sprintf("%v", v)
		}

		line := fmt.Sprintf("%d|%s|%s|%s|%d\n",
			cmd.Timestamp, cmd.Type, cmd.Key, valueStr, cmd.TTL.Nanoseconds())

		if _, err := mergedFile.WriteString(line); err != nil {
			fmt.Printf("Failed to write merged file: %v\n", err)
			return
		}
	}

	// Delete original files and rename merged file
	for _, filePath := range matches {
		if err := os.Remove(filePath); err != nil {
			fmt.Printf("Failed to remove file %s: %v\n", filePath, err)
		}
	}

	if err := os.Rename(mergedPath, pm.aclPath); err != nil {
		fmt.Printf("Failed to rename merged file: %v\n", err)
		return
	}

	// Reset file size
	pm.aclFileSize = 0
	if info, err := os.Stat(pm.aclPath); err == nil {
		pm.aclFileSize = info.Size()
	}

	fmt.Printf("ACL files compacted: %d commands merged into %d commands\n",
		len(matches), len(commands))
}

// SaveAtd 保存ATD快照（压缩二进制格式）
func (pm *PersistenceManager) SaveAtd() error {
	pm.mutex.Lock()
	defer pm.mutex.Unlock()

	if !pm.enabled {
		return nil
	}

	// 确保目录存在
	dir := filepath.Dir(pm.atdPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create directory: %v", err)
	}

	// 写入临时文件
	tempFile := pm.atdPath + ".tmp"
	file, err := os.Create(tempFile)
	if err != nil {
		return fmt.Errorf("failed to create temp file: %v", err)
	}

	// 创建gzip压缩器
	gzipWriter := gzip.NewWriter(file)
	writer := bufio.NewWriter(gzipWriter)

	// 写入文件头
	if err := pm.writeAtdHeader(writer); err != nil {
		writer.Flush()
		gzipWriter.Close()
		file.Close()
		return fmt.Errorf("failed to write header: %v", err)
	}

	// 写入缓存项
	pm.cache.mu.RLock()
	itemCount := 0
	for key, item := range pm.cache.items {
		// 检查是否过期
		if item.Expiration > 0 && time.Now().UnixNano() > item.Expiration {
			continue // 跳过过期数据
		}

		if err := pm.writeAtdItem(writer, key, item); err != nil {
			pm.cache.mu.RUnlock()
			writer.Flush()
			gzipWriter.Close()
			file.Close()
			return fmt.Errorf("failed to write item %s: %v", key, err)
		}
		itemCount++
	}

	// stats writing removed
	pm.cache.mu.RUnlock()

	// 写入结束标记
	if err := pm.writeAtdEndMarker(writer); err != nil {
		writer.Flush()
		gzipWriter.Close()
		file.Close()
		return fmt.Errorf("failed to write end marker: %v", err)
	}

	// 确保所有数据都写入
	writer.Flush()
	gzipWriter.Close()
	file.Close()

	// 原子性重命名
	if err := os.Rename(tempFile, pm.atdPath); err != nil {
		return fmt.Errorf("failed to rename temp file: %v", err)
	}

	pm.lastAtdTime = time.Now()
	fmt.Printf("ATD snapshot saved successfully with %d items at %s\n", itemCount, pm.lastAtdTime.Format("2006-01-02 15:04:05"))
	return nil
}

// LoadAtd 加载ATD快照
func (pm *PersistenceManager) LoadAtd() error {
	pm.mutex.Lock()
	defer pm.mutex.Unlock()

	if !pm.enabled {
		return nil
	}

	// 检查文件是否存在
	if _, err := os.Stat(pm.atdPath); os.IsNotExist(err) {
		return nil // 文件不存在，不是错误
	}

	// 读取文件
	file, err := os.Open(pm.atdPath)
	if err != nil {
		return fmt.Errorf("failed to open ATD file: %v", err)
	}
	defer file.Close()

	// 创建gzip解压器
	gzipReader, err := gzip.NewReader(file)
	if err != nil {
		return fmt.Errorf("failed to create gzip reader: %v", err)
	}
	defer gzipReader.Close()

	reader := bufio.NewReader(gzipReader)

	// 验证文件头
	if err := pm.readAtdHeader(reader); err != nil {
		return fmt.Errorf("failed to read header: %v", err)
	}

	// 恢复数据到缓存
	pm.cache.mu.Lock()
	defer pm.cache.mu.Unlock()

	// 清空现有数据
	pm.cache.items = make(map[string]*CacheItem)
	pm.cache.expirationHeap = &ExpirationHeap{}
	heap.Init(pm.cache.expirationHeap)

	// 读取记录
	itemCount := 0
	for {
		recordType, err := reader.ReadByte()
		if err != nil {
			if err == io.EOF {
				break
			}
			return fmt.Errorf("failed to read record type: %v", err)
		}

		switch recordType {
		case RECORD_ITEM:
			key, item, err := pm.readAtdItem(reader)
			if err != nil {
				return fmt.Errorf("failed to read item: %v", err)
			}
			if item != nil { // 跳过过期数据
				pm.cache.items[key] = item
				if item.Expiration > 0 {
					heap.Push(pm.cache.expirationHeap, item)
				}
				itemCount++
			}

		// RECORD_STATS case removed

		case RECORD_END:
			fmt.Printf("ATD snapshot loaded successfully with %d items\n", itemCount)
			return nil

		default:
			return fmt.Errorf("unknown record type: %d", recordType)
		}
	}

	return nil
}

// LoadAcl 加载ACL命令日志
func (pm *PersistenceManager) LoadAcl() error {
	pm.mutex.Lock()
	defer pm.mutex.Unlock()

	if !pm.enabled {
		return nil
	}

	// 检查文件是否存在
	if _, err := os.Stat(pm.aclPath); os.IsNotExist(err) {
		return nil // 文件不存在，不是错误
	}

	// 读取文件
	file, err := os.Open(pm.aclPath)
	if err != nil {
		return fmt.Errorf("failed to open ACL file: %v", err)
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	lineNum := 0
	commandCount := 0

	for scanner.Scan() {
		lineNum++
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}

		// 解析命令行: timestamp|type|key|value|ttl
		parts := strings.Split(line, "|")
		if len(parts) != 5 {
			fmt.Printf("Warning: Invalid ACL line %d: %s\n", lineNum, line)
			continue
		}

		// 解析时间戳（暂时不使用，但保留用于未来扩展）
		_, err := strconv.ParseInt(parts[0], 10, 64)
		if err != nil {
			fmt.Printf("Warning: Invalid timestamp in line %d: %s\n", lineNum, line)
			continue
		}

		// 解析TTL
		ttlNanos, err := strconv.ParseInt(parts[4], 10, 64)
		if err != nil {
			fmt.Printf("Warning: Invalid TTL in line %d: %s\n", lineNum, line)
			continue
		}
		ttl := time.Duration(ttlNanos)

		// 解析值
		var value interface{}
		cmdType := parts[1]
		key := parts[2]
		valueStr := parts[3]

		// 尝试解析复杂类型
		if strings.HasPrefix(valueStr, "[") && strings.HasSuffix(valueStr, "]") {
			// 数组格式: [apple banana orange]
			content := strings.Trim(valueStr, "[]")
			if content != "" {
				value = strings.Fields(content)
			} else {
				value = []string{}
			}
		} else if strings.HasPrefix(valueStr, "map[") && strings.Contains(valueStr, ":") {
			// 对象格式: map[age:25 name:john]
			content := strings.TrimPrefix(valueStr, "map[")
			content = strings.TrimSuffix(content, "]")
			pairs := strings.Fields(content)
			obj := make(map[string]string)
			for _, pair := range pairs {
				if strings.Contains(pair, ":") {
					kv := strings.SplitN(pair, ":", 2)
					if len(kv) == 2 {
						obj[kv[0]] = kv[1]
					}
				}
			}
			value = obj
		} else {
			// 普通字符串
			value = valueStr
		}

		// 执行命令 - 直接操作缓存，避免死锁
		switch cmdType {
		case CMD_SET, CMD_SETS, CMD_SETX:
			// 确定数据类型
			var dataType string
			switch value.(type) {
			case []string:
				dataType = "array"
			case map[string]string:
				dataType = "object"
			default:
				dataType = "string"
			}

			item := &CacheItem{
				Value: value,
				key:   key,
				Type:  dataType,
			}
			// Only set expiration time when ttl > 0
			if ttl > 0 {
				item.Expiration = time.Now().Add(ttl).UnixNano()
				// Add to expiration heap
				heap.Push(pm.cache.expirationHeap, item)
			}
			pm.cache.items[key] = item
			// stats tracking removed
		case CMD_DEL:
			// 删除字符串类型的key
			if item, exists := pm.cache.items[key]; exists && item.Type == "string" {
				// If has expiration time, remove from heap
				if item.Expiration > 0 && item.index >= 0 {
					heap.Remove(pm.cache.expirationHeap, item.index)
				}
				delete(pm.cache.items, key)
				// stats tracking removed
			}
		case CMD_DELS:
			// 删除数组类型的key
			if item, exists := pm.cache.items[key]; exists && item.Type == "array" {
				// If has expiration time, remove from heap
				if item.Expiration > 0 && item.index >= 0 {
					heap.Remove(pm.cache.expirationHeap, item.index)
				}
				delete(pm.cache.items, key)
				// stats tracking removed
			}
		case CMD_DELX:
			// 删除对象类型的key
			if item, exists := pm.cache.items[key]; exists && item.Type == "object" {
				// If has expiration time, remove from heap
				if item.Expiration > 0 && item.index >= 0 {
					heap.Remove(pm.cache.expirationHeap, item.index)
				}
				delete(pm.cache.items, key)
				// stats tracking removed
			}
		}
		commandCount++
	}

	if err := scanner.Err(); err != nil {
		return fmt.Errorf("failed to read ACL: %v", err)
	}

	fmt.Printf("ACL loaded successfully with %d commands\n", commandCount)
	return nil
}

// writeAtdHeader 写入ATD文件头
func (pm *PersistenceManager) writeAtdHeader(writer *bufio.Writer) error {
	// Magic number
	if err := binary.Write(writer, binary.BigEndian, MAGIC_HEADER); err != nil {
		return fmt.Errorf("failed to write magic number: %v", err)
	}
	// Version
	if err := writer.WriteByte(VERSION); err != nil {
		return fmt.Errorf("failed to write version: %v", err)
	}
	// Timestamp
	timestamp := int64(time.Now().Unix())
	if err := binary.Write(writer, binary.BigEndian, timestamp); err != nil {
		return fmt.Errorf("failed to write timestamp: %v", err)
	}
	return nil
}

// readAtdHeader 读取ATD文件头
func (pm *PersistenceManager) readAtdHeader(reader *bufio.Reader) error {
	// Magic number
	var magic uint32
	if err := binary.Read(reader, binary.BigEndian, &magic); err != nil {
		return err
	}
	if magic != MAGIC_HEADER {
		return fmt.Errorf("invalid magic number: %x", magic)
	}

	// Version
	version, err := reader.ReadByte()
	if err != nil {
		return err
	}
	if version != VERSION {
		return fmt.Errorf("unsupported version: %d", version)
	}

	// Timestamp (skip)
	var timestamp int64
	if err := binary.Read(reader, binary.BigEndian, &timestamp); err != nil {
		return err
	}

	return nil
}

// writeAtdItem 写入ATD缓存项
func (pm *PersistenceManager) writeAtdItem(writer *bufio.Writer, key string, item *CacheItem) error {
	// Record type
	if err := writer.WriteByte(RECORD_ITEM); err != nil {
		return err
	}

	// Key length and key
	keyBytes := []byte(key)
	if err := binary.Write(writer, binary.BigEndian, uint16(len(keyBytes))); err != nil {
		return err
	}
	if _, err := writer.Write(keyBytes); err != nil {
		return err
	}

	// Value type and value
	if err := pm.writeAtdValue(writer, item.Value); err != nil {
		return err
	}

	// Expiration time
	if err := binary.Write(writer, binary.BigEndian, item.Expiration); err != nil {
		return err
	}

	return nil
}

// readAtdItem 读取ATD缓存项
func (pm *PersistenceManager) readAtdItem(reader *bufio.Reader) (string, *CacheItem, error) {
	// Key length and key
	var keyLen uint16
	if err := binary.Read(reader, binary.BigEndian, &keyLen); err != nil {
		return "", nil, err
	}

	keyBytes := make([]byte, keyLen)
	if _, err := io.ReadFull(reader, keyBytes); err != nil {
		return "", nil, err
	}
	key := string(keyBytes)

	// Value
	value, err := pm.readAtdValue(reader)
	if err != nil {
		return "", nil, err
	}

	// Expiration time
	var expiration int64
	if err := binary.Read(reader, binary.BigEndian, &expiration); err != nil {
		return "", nil, err
	}

	// 检查是否过期
	if expiration > 0 && time.Now().UnixNano() > expiration {
		return key, nil, nil // 返回nil表示跳过过期数据
	}

	item := &CacheItem{
		Value:      value,
		Expiration: expiration,
		key:        key,
	}

	return key, item, nil
}

// writeAtdValue 写入ATD值
func (pm *PersistenceManager) writeAtdValue(writer *bufio.Writer, value interface{}) error {
	switch v := value.(type) {
	case string:
		if err := writer.WriteByte(0x01); err != nil { // String type
			return err
		}
		valueBytes := []byte(v)
		if err := binary.Write(writer, binary.BigEndian, uint32(len(valueBytes))); err != nil {
			return err
		}
		_, err := writer.Write(valueBytes)
		return err

	case []string:
		if err := writer.WriteByte(0x02); err != nil { // Array type
			return err
		}
		if err := binary.Write(writer, binary.BigEndian, uint16(len(v))); err != nil {
			return err
		}
		for _, s := range v {
			valueBytes := []byte(s)
			if err := binary.Write(writer, binary.BigEndian, uint16(len(valueBytes))); err != nil {
				return err
			}
			if _, err := writer.Write(valueBytes); err != nil {
				return err
			}
		}
		return nil

	case map[string]string:
		if err := writer.WriteByte(0x03); err != nil { // Object type
			return err
		}
		if err := binary.Write(writer, binary.BigEndian, uint16(len(v))); err != nil {
			return err
		}
		for k, s := range v {
			// Key
			keyBytes := []byte(k)
			if err := binary.Write(writer, binary.BigEndian, uint16(len(keyBytes))); err != nil {
				return err
			}
			if _, err := writer.Write(keyBytes); err != nil {
				return err
			}
			// Value
			valueBytes := []byte(s)
			if err := binary.Write(writer, binary.BigEndian, uint16(len(valueBytes))); err != nil {
				return err
			}
			if _, err := writer.Write(valueBytes); err != nil {
				return err
			}
		}
		return nil

	default:
		// 对于其他类型，转换为字符串
		if err := writer.WriteByte(0x01); err != nil { // String type
			return err
		}
		valueStr := fmt.Sprintf("%v", v)
		valueBytes := []byte(valueStr)
		if err := binary.Write(writer, binary.BigEndian, uint32(len(valueBytes))); err != nil {
			return err
		}
		_, err := writer.Write(valueBytes)
		return err
	}
}

// readAtdValue 读取ATD值
func (pm *PersistenceManager) readAtdValue(reader *bufio.Reader) (interface{}, error) {
	valueType, err := reader.ReadByte()
	if err != nil {
		return nil, err
	}

	switch valueType {
	case 0x01: // String
		var length uint32
		if err := binary.Read(reader, binary.BigEndian, &length); err != nil {
			return nil, err
		}
		valueBytes := make([]byte, length)
		if _, err := io.ReadFull(reader, valueBytes); err != nil {
			return nil, err
		}
		return string(valueBytes), nil

	case 0x02: // Array
		var length uint16
		if err := binary.Read(reader, binary.BigEndian, &length); err != nil {
			return nil, err
		}
		array := make([]string, length)
		for i := uint16(0); i < length; i++ {
			var strLen uint16
			if err := binary.Read(reader, binary.BigEndian, &strLen); err != nil {
				return nil, err
			}
			strBytes := make([]byte, strLen)
			if _, err := io.ReadFull(reader, strBytes); err != nil {
				return nil, err
			}
			array[i] = string(strBytes)
		}
		return array, nil

	case 0x03: // Object
		var length uint16
		if err := binary.Read(reader, binary.BigEndian, &length); err != nil {
			return nil, err
		}
		object := make(map[string]string)
		for i := uint16(0); i < length; i++ {
			// Key
			var keyLen uint16
			if err := binary.Read(reader, binary.BigEndian, &keyLen); err != nil {
				return nil, err
			}
			keyBytes := make([]byte, keyLen)
			if _, err := io.ReadFull(reader, keyBytes); err != nil {
				return nil, err
			}
			// Value
			var valueLen uint16
			if err := binary.Read(reader, binary.BigEndian, &valueLen); err != nil {
				return nil, err
			}
			valueBytes := make([]byte, valueLen)
			if _, err := io.ReadFull(reader, valueBytes); err != nil {
				return nil, err
			}
			object[string(keyBytes)] = string(valueBytes)
		}
		return object, nil

	default:
		return nil, fmt.Errorf("unknown value type: %d", valueType)
	}
}

// writeAtdStats method removed

// readAtdStats method removed

// writeAtdEndMarker 写入ATD结束标记
func (pm *PersistenceManager) writeAtdEndMarker(writer *bufio.Writer) error {
	return writer.WriteByte(RECORD_END)
}

// periodicAtd 定期保存ATD快照
func (pm *PersistenceManager) periodicAtd() {
	ticker := time.NewTicker(pm.atdInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			if err := pm.SaveAtd(); err != nil {
				fmt.Printf("Failed to save ATD: %v\n", err)
			}
		case <-pm.stopChan:
			return
		}
	}
}

// periodicAcl 定期同步ACL
func (pm *PersistenceManager) periodicAcl() {
	ticker := time.NewTicker(pm.aclInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			pm.flushAcl()
		case <-pm.stopChan:
			return
		}
	}
}

// GetLastAtdTime 获取最后ATD保存时间
func (pm *PersistenceManager) GetLastAtdTime() time.Time {
	pm.mutex.RLock()
	defer pm.mutex.RUnlock()
	return pm.lastAtdTime
}

// IsEnabled 检查是否启用
func (pm *PersistenceManager) IsEnabled() bool {
	pm.mutex.RLock()
	defer pm.mutex.RUnlock()
	return pm.enabled
}

// SetEnabled 设置启用状态
func (pm *PersistenceManager) SetEnabled(enabled bool) {
	pm.mutex.Lock()
	defer pm.mutex.Unlock()
	pm.enabled = enabled
}
