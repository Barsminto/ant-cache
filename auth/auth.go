package auth

import (
	"bufio"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"strings"
	"syscall"

	"golang.org/x/crypto/pbkdf2"
	"golang.org/x/term"
)

const (
	SaltLength = 32
	KeyLength  = 32
	Iterations = 100000
)

type AuthManager struct {
	passwordFile string
	enabled      bool
}

type PasswordData struct {
	Salt string `json:"salt"`
	Hash string `json:"hash"`
}

func NewAuthManager(passwordFile string, enabled bool) *AuthManager {
	return &AuthManager{
		passwordFile: passwordFile,
		enabled:      enabled,
	}
}

func (am *AuthManager) IsEnabled() bool {
	return am.enabled
}

func (am *AuthManager) HasPassword() bool {
	if !am.enabled {
		return false
	}
	_, err := os.Stat(am.passwordFile)
	return err == nil
}

func (am *AuthManager) SetPassword(password string) error {
	if !am.enabled {
		return fmt.Errorf("authentication is disabled")
	}

	// Generate random salt
	salt := make([]byte, SaltLength)
	if _, err := rand.Read(salt); err != nil {
		return fmt.Errorf("failed to generate salt: %v", err)
	}

	// Generate password hash using PBKDF2
	hash := pbkdf2.Key([]byte(password), salt, Iterations, KeyLength, sha256.New)

	// 保存到文件
	file, err := os.Create(am.passwordFile)
	if err != nil {
		return fmt.Errorf("failed to create password file: %v", err)
	}
	defer file.Close()

	// Write salt and hash (hexadecimal format)
	saltHex := hex.EncodeToString(salt)
	hashHex := hex.EncodeToString(hash)

	if _, err := fmt.Fprintf(file, "%s\n%s\n", saltHex, hashHex); err != nil {
		return fmt.Errorf("failed to write password file: %v", err)
	}

	return nil
}

func (am *AuthManager) VerifyPassword(password string) (bool, error) {
	if !am.enabled {
		return true, nil
	}

	if !am.HasPassword() {
		return false, fmt.Errorf("no password set")
	}

	// Read password file
	file, err := os.Open(am.passwordFile)
	if err != nil {
		return false, fmt.Errorf("failed to open password file: %v", err)
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)

	// Read salt
	if !scanner.Scan() {
		return false, fmt.Errorf("invalid password file format")
	}
	saltHex := strings.TrimSpace(scanner.Text())

	// Read hash
	if !scanner.Scan() {
		return false, fmt.Errorf("invalid password file format")
	}
	hashHex := strings.TrimSpace(scanner.Text())

	// Decode hexadecimal
	salt, err := hex.DecodeString(saltHex)
	if err != nil {
		return false, fmt.Errorf("invalid salt format: %v", err)
	}

	expectedHash, err := hex.DecodeString(hashHex)
	if err != nil {
		return false, fmt.Errorf("invalid hash format: %v", err)
	}

	// Calculate hash of input password
	actualHash := pbkdf2.Key([]byte(password), salt, Iterations, KeyLength, sha256.New)

	// 比较哈希
	if len(actualHash) != len(expectedHash) {
		return false, nil
	}

	for i := range actualHash {
		if actualHash[i] != expectedHash[i] {
			return false, nil
		}
	}

	return true, nil
}

func (am *AuthManager) PromptPassword(prompt string) (string, error) {
	fmt.Print(prompt)

	// Hide input
	bytePassword, err := term.ReadPassword(int(syscall.Stdin))
	if err != nil {
		return "", fmt.Errorf("failed to read password: %v", err)
	}

	fmt.Println() // New line
	return string(bytePassword), nil
}

func (am *AuthManager) SetupPassword() error {
	if !am.enabled {
		return fmt.Errorf("authentication is disabled")
	}

	fmt.Println("Setting up password protection...")

	password, err := am.PromptPassword("Enter new password: ")
	if err != nil {
		return err
	}

	if len(password) < 6 {
		return fmt.Errorf("password must be at least 6 characters long")
	}

	confirmPassword, err := am.PromptPassword("Confirm password: ")
	if err != nil {
		return err
	}

	if password != confirmPassword {
		return fmt.Errorf("passwords do not match")
	}

	if err := am.SetPassword(password); err != nil {
		return fmt.Errorf("failed to set password: %v", err)
	}

	fmt.Println("Password set successfully!")
	return nil
}

func (am *AuthManager) Authenticate() error {
	if !am.enabled {
		return nil
	}

	if !am.HasPassword() {
		return am.SetupPassword()
	}

	maxAttempts := 3
	for attempt := 1; attempt <= maxAttempts; attempt++ {
		password, err := am.PromptPassword("Enter password: ")
		if err != nil {
			return err
		}

		valid, err := am.VerifyPassword(password)
		if err != nil {
			return fmt.Errorf("authentication error: %v", err)
		}

		if valid {
			fmt.Println("Authentication successful!")
			return nil
		}

		if attempt < maxAttempts {
			fmt.Printf("Invalid password. %d attempts remaining.\n", maxAttempts-attempt)
		}
	}

	return fmt.Errorf("authentication failed: maximum attempts exceeded")
}
