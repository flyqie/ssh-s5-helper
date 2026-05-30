package proxy

import (
	"context"
	"errors"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/things-go/go-socks5"
	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/knownhosts"
)

const (
	logFileName = "ssh_s5_helper.log"
	maxLogSize  = 2 * 1024 * 1024
)

// remoteResolver delegates DNS resolution to the SSH server-side,
// enabling remote DNS resolution for domain names.
type remoteResolver struct{}

func (r *remoteResolver) Resolve(ctx context.Context, name string) (context.Context, net.IP, error) {
	// Return nil IP - the SOCKS5 library will use the FQDN directly when dialing.
	// Since the dial goes through sshClient.Dial() which tunnels through SSH,
	// DNS resolution happens on the remote SSH server side.
	return ctx, nil, nil
}

type Server struct {
	socks5Server *socks5.Server
	sshClient    *ssh.Client
	config       Config
	log          *Logger
	mu           struct {
		sync.RWMutex
		running bool
	}
}

type Config struct {
	SSHHost           string
	SSHPort           int
	SSHUser           string
	SSHPassword       string
	ForceCheckHostKey bool
	SOCKS5Listen      string
}

func New(cfg Config) (*Server, error) {
	log, err := NewLogger()
	if err != nil {
		return nil, fmt.Errorf("init logger: %w", err)
	}

	sshClient, err := newSSHClient(cfg, log)
	if err != nil {
		log.Printf("[ERROR] Failed to connect SSH %s:%d -> %v", cfg.SSHHost, cfg.SSHPort, err)
		return nil, err
	}

	log.Printf("[INFO] SSH connection established %s:%d (user: %s)", cfg.SSHHost, cfg.SSHPort, cfg.SSHUser)

	socks5Server := socks5.NewServer(
		socks5.WithResolver(&remoteResolver{}),
		socks5.WithDial(func(_ context.Context, network, addr string) (net.Conn, error) {
			return sshClient.Dial(network, addr)
		}),
	)

	return &Server{
		socks5Server: socks5Server,
		sshClient:    sshClient,
		config:       cfg,
		log:          log,
	}, nil
}

func (s *Server) Run(ctx context.Context) error {
	ln, err := net.Listen("tcp", s.config.SOCKS5Listen)
	if err != nil {
		s.log.Printf("[ERROR] Failed to listen on %s -> %v", s.config.SOCKS5Listen, err)
		return fmt.Errorf("listen: %w", err)
	}

	go func() {
		<-ctx.Done()
		ln.Close()
	}()

	s.mu.Lock()
	s.mu.running = true
	s.mu.Unlock()

	s.log.Printf("[INFO] Server started on %s", s.config.SOCKS5Listen)
	s.log.Printf("[INFO] Proxy config: ssh=%s:%d socks5=%s check_host_key=%v",
		s.config.SSHHost, s.config.SSHPort, s.config.SOCKS5Listen, s.config.ForceCheckHostKey)

	err = s.socks5Server.Serve(ln)
	if !errors.Is(err, net.ErrClosed) {
		return err
	}
	return nil
}

func (s *Server) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.mu.running = false

	if s.sshClient != nil {
		s.log.Printf("[INFO] Server stopped")
		return s.sshClient.Close()
	}
	return nil
}

func (s *Server) IsRunning() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.mu.running
}

type Logger struct {
	file *os.File
	path string
	mu   sync.Mutex
}

func NewLogger() (*Logger, error) {
	exePath, err := os.Executable()
	if err != nil {
		return nil, err
	}
	exeDir := filepath.Dir(exePath)
	logPath := filepath.Join(exeDir, logFileName)

	f, err := os.OpenFile(logPath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		return nil, err
	}

	return &Logger{file: f, path: logPath}, nil
}

func (l *Logger) Printf(format string, args ...interface{}) {
	l.mu.Lock()
	defer l.mu.Unlock()

	if info, err := l.file.Stat(); err == nil && info.Size() > maxLogSize {
		l.file.Close()
		os.Truncate(l.path, 0)
		l.file, _ = os.OpenFile(l.path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
	}

	t := time.Now().Format("2006-01-02 15:04:05")
	l.file.WriteString(fmt.Sprintf("[%s] "+format+"\n", append([]interface{}{t}, args...)...))
	l.file.Sync()
}

func (l *Logger) Close() error {
	return l.file.Close()
}

func newSSHClient(cfg Config, log *Logger) (*ssh.Client, error) {
	if cfg.SSHPassword == "" {
		return nil, fmt.Errorf("password is required")
	}

	addr := fmt.Sprintf("%s:%d", cfg.SSHHost, cfg.SSHPort)

	var hostKeyCallback ssh.HostKeyCallback
	var hostKeyMode string
	if cfg.ForceCheckHostKey {
		homeDir := os.Getenv("HOME")
		if homeDir == "" {
			homeDir = os.Getenv("USERPROFILE")
		}
		knownHostsPath := filepath.Join(homeDir, ".ssh", "known_hosts")
		callback, err := knownhosts.New(knownHostsPath)
		if err != nil {
			return nil, fmt.Errorf("known_hosts not found or invalid: %w", err)
		}
		hostKeyCallback = callback
		hostKeyMode = "verified"
	} else {
		hostKeyCallback = ssh.InsecureIgnoreHostKey()
		hostKeyMode = "skip"
	}

	log.Printf("[DEBUG] Connecting SSH: addr=%s:%d user=%s auth=password host_key=%s",
		cfg.SSHHost, cfg.SSHPort, cfg.SSHUser, hostKeyMode)

	client, err := ssh.Dial("tcp", addr, &ssh.ClientConfig{
		User:            cfg.SSHUser,
		Auth:            []ssh.AuthMethod{ssh.Password(cfg.SSHPassword)},
		HostKeyCallback: hostKeyCallback,
	})
	if err != nil {
		log.Printf("[ERROR] SSH dial failed: addr=%s:%d user=%s error=%v",
			cfg.SSHHost, cfg.SSHPort, cfg.SSHUser, err)
		return nil, fmt.Errorf("ssh dial: %w", err)
	}

	log.Printf("[INFO] SSH dial success: addr=%s:%d user=%s", cfg.SSHHost, cfg.SSHPort, cfg.SSHUser)

	return client, nil
}