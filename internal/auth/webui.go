// Package auth WebUI 管理端认证：
//   - 首次访问设置密码（SHA256(salt+password) 哈希存储，不保存明文）
//   - 删除 storage/webui_auth.json 即重置密码
//   - 登录成功签发 2 小时内存令牌，重启后全部失效
package auth

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

const (
	tokenTTL    = 2 * time.Hour
	saltBytes   = 16
	tokenBytes  = 32
)

var (
	ErrNotSetup      = errors.New("尚未设置密码")
	ErrAlreadySetup  = errors.New("密码已设置，请先删除认证文件以重置")
	ErrBadPassword   = errors.New("密码错误")
	ErrTokenInvalid  = errors.New("令牌无效或已过期")
)

type credential struct {
	Salt string `json:"salt"`
	Hash string `json:"hash"` // hex(sha256(salt || password))
}

type tokenEntry struct {
	Expire time.Time
}

// Store WebUI 认证仓库。
type Store struct {
	mu     sync.Mutex
	path   string
	cred   *credential
	tokens map[string]tokenEntry
}

// NewStore 加载认证文件。文件不存在 = 未初始化（IsInitialized()==false）。
func NewStore(path string) (*Store, error) {
	s := &Store{path: path, tokens: map[string]tokenEntry{}}
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return s, nil
		}
		return nil, err
	}
	var c credential
	if json.Unmarshal(data, &c) == nil && c.Salt != "" && c.Hash != "" {
		s.cred = &c
	}
	return s, nil
}

// IsInitialized 是否已设置密码。
func (s *Store) IsInitialized() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.cred != nil
}

func hashPassword(salt, password string) string {
	h := sha256.Sum256([]byte(salt + password))
	return hex.EncodeToString(h[:])
}

// Setup 首次设置密码。
func (s *Store) Setup(password string) error {
	if len(password) < 6 {
		return errors.New("密码至少 6 位")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.cred != nil {
		return ErrAlreadySetup
	}
	sb := make([]byte, saltBytes)
	if _, err := rand.Read(sb); err != nil {
		return err
	}
	salt := hex.EncodeToString(sb)
	c := &credential{Salt: salt, Hash: hashPassword(salt, password)}
	data, _ := json.MarshalIndent(c, "", "  ")
	_ = os.MkdirAll(filepath.Dir(s.path), 0o755)
	if err := os.WriteFile(s.path, data, 0o600); err != nil {
		return fmt.Errorf("写入认证文件失败: %w", err)
	}
	s.cred = c
	return nil
}

// Reset 删除认证文件并吊销全部令牌（重置密码）。
func (s *Store) Reset() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.cred = nil
	s.tokens = map[string]tokenEntry{}
	err := os.Remove(s.path)
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}

// Login 校验密码并签发令牌。
func (s *Store) Login(password string) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.cred == nil {
		return "", ErrNotSetup
	}
	if hashPassword(s.cred.Salt, password) != s.cred.Hash {
		return "", ErrBadPassword
	}
	tb := make([]byte, tokenBytes)
	if _, err := rand.Read(tb); err != nil {
		return "", err
	}
	token := hex.EncodeToString(tb)
	s.tokens[token] = tokenEntry{Expire: time.Now().Add(tokenTTL)}
	s.gcLocked()
	return token, nil
}

// Validate 校验令牌有效性。
func (s *Store) Validate(token string) bool {
	if token == "" {
		return false
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	e, ok := s.tokens[token]
	if !ok || time.Now().After(e.Expire) {
		delete(s.tokens, token)
		return false
	}
	return true
}

// Revoke 吊销单个令牌。
func (s *Store) Revoke(token string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.tokens, token)
}

// gcLocked 清理过期令牌（需持锁）。
func (s *Store) gcLocked() {
	now := time.Now()
	for t, e := range s.tokens {
		if now.After(e.Expire) {
			delete(s.tokens, t)
		}
	}
}
