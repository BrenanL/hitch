package credentials

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"

	"filippo.io/age"
	"filippo.io/age/armor"
)

// Store manages encrypted credential storage.
type Store struct {
	path       string
	passphrase string
	mu         sync.Mutex
	cache      map[string]string
}

// NewStore creates a credential store at the given path.
func NewStore(path string) *Store {
	return &Store{
		path: path,
	}
}

// SetPassphrase sets the passphrase for encrypting/decrypting.
// This is called once per session after prompting the user.
func (s *Store) SetPassphrase(passphrase string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.passphrase = passphrase
	s.cache = nil // invalidate cache on passphrase change
}

// Get retrieves a credential by key.
// It checks env var fallback first, then the encrypted store.
func (s *Store) Get(key string) (string, error) {
	// Check env var fallback first
	if val := EnvGet(key); val != "" {
		return val, nil
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	// Load and cache if needed
	if s.cache == nil {
		if err := s.load(); err != nil {
			return "", err
		}
	}

	return s.cache[key], nil
}

// Set stores a credential.
func (s *Store) Set(key, value string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.passphrase == "" {
		return fmt.Errorf("passphrase not set — call SetPassphrase first")
	}

	if s.cache == nil {
		if err := s.load(); err != nil && !os.IsNotExist(err) {
			return err
		}
		if s.cache == nil {
			s.cache = make(map[string]string)
		}
	}

	s.cache[key] = value
	return s.save()
}

// Delete removes a credential.
func (s *Store) Delete(key string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.passphrase == "" {
		return fmt.Errorf("passphrase not set — call SetPassphrase first")
	}

	if s.cache == nil {
		if err := s.load(); err != nil {
			return err
		}
	}

	delete(s.cache, key)
	return s.save()
}

// List returns all credential keys.
func (s *Store) List() ([]string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.cache == nil {
		if err := s.load(); err != nil {
			return nil, err
		}
	}

	keys := make([]string, 0, len(s.cache))
	for k := range s.cache {
		keys = append(keys, k)
	}
	return keys, nil
}

func (s *Store) load() error {
	data, err := os.ReadFile(s.path)
	if os.IsNotExist(err) {
		s.cache = make(map[string]string)
		return nil
	}
	if err != nil {
		return fmt.Errorf("reading credentials file: %w", err)
	}

	if s.passphrase == "" {
		return fmt.Errorf("passphrase not set — call SetPassphrase first")
	}

	identity, err := age.NewScryptIdentity(s.passphrase)
	if err != nil {
		return fmt.Errorf("creating identity: %w", err)
	}

	armorReader := armor.NewReader(bytes.NewReader(data))
	reader, err := age.Decrypt(armorReader, identity)
	if err != nil {
		return fmt.Errorf("decrypting credentials: %w", err)
	}

	plaintext, err := io.ReadAll(reader)
	if err != nil {
		return fmt.Errorf("reading decrypted data: %w", err)
	}

	s.cache = make(map[string]string)
	if err := json.Unmarshal(plaintext, &s.cache); err != nil {
		return fmt.Errorf("parsing credentials: %w", err)
	}

	return nil
}

func (s *Store) save() error {
	data, err := json.Marshal(s.cache)
	if err != nil {
		return fmt.Errorf("marshaling credentials: %w", err)
	}

	recipient, err := age.NewScryptRecipient(s.passphrase)
	if err != nil {
		return fmt.Errorf("creating recipient: %w", err)
	}

	var buf bytes.Buffer
	armorWriter := armor.NewWriter(&buf)
	writer, err := age.Encrypt(armorWriter, recipient)
	if err != nil {
		return fmt.Errorf("creating encryptor: %w", err)
	}

	if _, err := writer.Write(data); err != nil {
		return fmt.Errorf("writing encrypted data: %w", err)
	}
	if err := writer.Close(); err != nil {
		return fmt.Errorf("closing encryptor: %w", err)
	}
	if err := armorWriter.Close(); err != nil {
		return fmt.Errorf("closing armor writer: %w", err)
	}

	dir := filepath.Dir(s.path)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return fmt.Errorf("creating directory: %w", err)
	}

	if err := os.WriteFile(s.path, buf.Bytes(), 0o600); err != nil {
		return fmt.Errorf("writing credentials file: %w", err)
	}

	return nil
}
