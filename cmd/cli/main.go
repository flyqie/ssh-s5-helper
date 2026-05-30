package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/flyqie/ssh-s5-helper/internal/config"
	"github.com/flyqie/ssh-s5-helper/internal/proxy"
	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "ssh-s5-helper",
	Short: "SSH SOCKS5 proxy helper",
	Run:   run,
}

var args config.Args

func main() {
	rootCmd.Flags().StringVarP(&args.SSHHost, "ssh-host", "H", "", "SSH host")
	rootCmd.Flags().IntVarP(&args.SSHPort, "ssh-port", "p", 0, "SSH port")
	rootCmd.Flags().StringVarP(&args.SSHUser, "ssh-user", "u", "", "SSH user")
	rootCmd.Flags().StringVarP(&args.SSHPassword, "ssh-password", "P", "", "SSH password")
	rootCmd.Flags().StringVarP(&args.SOCKS5Listen, "listen", "l", "", "SOCKS5 listen address")
	rootCmd.Flags().BoolVar(&args.ForceCheckHostKey, "force-check-host-key", false, "Force check host key")

	rootCmd.Execute()
}

func log(format string, args ...interface{}) {
	t := time.Now().Format("2006-01-02 15:04:05")
	fmt.Printf("[%s] "+format+"\n", append([]interface{}{t}, args...)...)
}

func run(cmd *cobra.Command, _ []string) {
	loader := config.NewLoader()
	cfg, err := loader.Load(&args)
	if err != nil {
		log("[ERROR] Load config: %v", err)
		os.Exit(1)
	}

	if cfg.SSH.Host == "" {
		err := fmt.Errorf("SSH host is required")
		log("[ERROR] %v", err)
		os.Exit(1)
	}
	if cfg.SSH.User == "" {
		err := fmt.Errorf("SSH user is required")
		log("[ERROR] %v", err)
		os.Exit(1)
	}
	if cfg.SSH.Password == "" {
		err := fmt.Errorf("SSH password is required")
		log("[ERROR] %v", err)
		os.Exit(1)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	server, err := proxy.New(proxy.Config{
		SSHHost:           cfg.SSH.Host,
		SSHPort:           cfg.SSH.Port,
		SSHUser:           cfg.SSH.User,
		SSHPassword:       cfg.SSH.Password,
		ForceCheckHostKey: cfg.ForceCheckHostKey,
		SOCKS5Listen:      cfg.SOCKS5.ListenAddr,
	})
	if err != nil {
		log("[ERROR] Create server: %v", err)
		os.Exit(1)
	}
	defer server.Close()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		<-sigCh
		cancel()
	}()

	if config.SWVersion != "" {
		log("[INFO] Software Version: %s", config.SWVersion)
	}
	log("[INFO] SSH SOCKS5 Proxy started")
	log("[INFO] SSH: %s:%d (user: %s)", cfg.SSH.Host, cfg.SSH.Port, cfg.SSH.User)
	log("[INFO] SOCKS5 listen: %s", cfg.SOCKS5.ListenAddr)

	if err := server.Run(ctx); err != nil {
		log("[ERROR] Server: %v", err)
		os.Exit(1)
	}
}
