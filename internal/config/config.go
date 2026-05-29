package config

import (
	"crypto/aes"
	"crypto/cipher"
	"encoding/base64"
	"os"
	"path/filepath"
	"strings"

	"github.com/pelletier/go-toml/v2"
)

var encryptKey = "helloworld"

type Config struct {
	SSH               SSHConfig
	SOCKS5            SOCKS5Config
	ForceCheckHostKey bool
}

type SSHConfig struct {
	Host     string
	Port     int
	User     string
	Password string
}

type SOCKS5Config struct {
	ListenAddr string
}

var (
	sshAddr           string
	sshUser           string
	sshPassword       string
	socks5Addr        string
	forceCheckHostKey bool
)

type Args struct {
	SSHHost             string
	SSHPort             int
	SSHUser             string
	SSHPassword         string
	SOCKS5Listen        string
	ForceCheckHostKey   bool
}

type Loader struct{}

func NewLoader() *Loader {
	return &Loader{}
}

func (l *Loader) Load(args *Args) (*Config, error) {
	cfg := defaultConfig()

	l.loadFromFile(cfg)
	l.loadFromEnv(cfg)
	l.applyArgs(cfg, args)

	return cfg, nil
}

func defaultConfig() *Config {
	cfg := &Config{
		SSH:               SSHConfig{Port: 22},
		SOCKS5:            SOCKS5Config{ListenAddr: "127.0.0.1:1080"},
		ForceCheckHostKey: forceCheckHostKey,
	}

	if sshAddr != "" {
		if host, port := splitHostPort(sshAddr); host != "" {
			cfg.SSH.Host = host
			cfg.SSH.Port = port
		}
	}
	if sshUser != "" {
		cfg.SSH.User = sshUser
	}
	if sshPassword != "" {
		cfg.SSH.Password = sshPassword
	}
	if socks5Addr != "" {
		cfg.SOCKS5.ListenAddr = socks5Addr
	}

	return cfg
}

func splitHostPort(addr string) (string, int) {
	for i := len(addr) - 1; i >= 0; i-- {
		if addr[i] == ':' {
			return addr[:i], parseInt(addr[i+1:], 22)
		}
	}
	return addr, 22
}

func (l *Loader) loadFromEnv(cfg *Config) {
	if v := os.Getenv("SSH_S5_HELPER_HOST"); v != "" {
		cfg.SSH.Host = v
	}
	if v := os.Getenv("SSH_S5_HELPER_PORT"); v != "" {
		cfg.SSH.Port = parseInt(v, 0)
	}
	if v := os.Getenv("SSH_S5_HELPER_USER"); v != "" {
		cfg.SSH.User = v
	}
	if v := os.Getenv("SSH_S5_HELPER_PASSWORD"); v != "" {
		cfg.SSH.Password = v
	}
	if v := os.Getenv("SSH_S5_HELPER_SOCKS5_LISTEN"); v != "" {
		cfg.SOCKS5.ListenAddr = v
	}
}

func (l *Loader) loadFromFile(cfg *Config) {
	exePath, err := os.Executable()
	if err != nil {
		return
	}
	exeDir := filepath.Dir(exePath)
	data, err := os.ReadFile(filepath.Join(exeDir, "config.toml"))
	if err != nil {
		return
	}

	content := string(data)
	if strings.HasPrefix(content, "ENCRYPTED") {
		encrypted := strings.TrimPrefix(content, "ENCRYPTED")
		decrypted, err := aesDecrypt(strings.TrimSpace(encrypted))
		if err != nil {
			return
		}
		content = decrypted
	}

	var fileCfg Config
	if err := toml.Unmarshal([]byte(content), &fileCfg); err == nil {
		mergeConfig(cfg, &fileCfg)
	}
}

func (l *Loader) applyArgs(cfg *Config, args *Args) {
	if args == nil {
		return
	}
	if args.SSHHost != "" {
		cfg.SSH.Host = args.SSHHost
	}
	if args.SSHPort > 0 {
		cfg.SSH.Port = args.SSHPort
	}
	if args.SSHUser != "" {
		cfg.SSH.User = args.SSHUser
	}
	if args.SSHPassword != "" {
		cfg.SSH.Password = args.SSHPassword
	}
	if args.SOCKS5Listen != "" {
		cfg.SOCKS5.ListenAddr = args.SOCKS5Listen
	}
	if args.ForceCheckHostKey {
		cfg.ForceCheckHostKey = true
	}
}

func mergeConfig(dst, src *Config) {
	if src.SSH.Host != "" {
		dst.SSH.Host = src.SSH.Host
	}
	if src.SSH.Port != 0 {
		dst.SSH.Port = src.SSH.Port
	}
	if src.SSH.User != "" {
		dst.SSH.User = src.SSH.User
	}
	if src.SSH.Password != "" {
		dst.SSH.Password = src.SSH.Password
	}
	if src.SOCKS5.ListenAddr != "" {
		dst.SOCKS5.ListenAddr = src.SOCKS5.ListenAddr
	}
}

func parseInt(s string, defaultVal int) int {
	if s == "" {
		return defaultVal
	}
	n := 0
	for _, c := range s {
		if c < '0' || c > '9' {
			return defaultVal
		}
		n = n*10 + int(c-'0')
	}
	return n
}

func aesDecrypt(data string) (string, error) {
	key := []byte(encryptKey)
	if len(key) != 16 {
		return "", nil
	}

	ciphertext, err := base64.StdEncoding.DecodeString(data)
	if err != nil {
		return "", err
	}

	block, err := aes.NewCipher(key)
	if err != nil {
		return "", err
	}

	if len(ciphertext) < aes.BlockSize {
		return "", nil
	}

	iv := ciphertext[:aes.BlockSize]
	ciphertext = ciphertext[aes.BlockSize:]

	stream := cipher.NewCFBDecrypter(block, iv)
	stream.XORKeyStream(ciphertext, ciphertext)

	return string(ciphertext), nil
}