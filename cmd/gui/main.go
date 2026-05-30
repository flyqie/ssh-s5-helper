package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/flyqie/ssh-s5-helper/internal/config"
	"github.com/flyqie/ssh-s5-helper/internal/gui"
	"github.com/flyqie/ssh-s5-helper/internal/proxy"
	"github.com/getlantern/systray"
	"golang.design/x/clipboard"
)

func main() {
	if err := clipboard.Init(); err != nil {
		panic(err)
	}

	loader := config.NewLoader()
	cfg, _ := loader.Load(nil)

	app := gui.New()
	app.SetListenAddr(cfg.SOCKS5.ListenAddr)

	socks5URL := func() string {
		return fmt.Sprintf("socks5://%s", cfg.SOCKS5.ListenAddr)
	}

	ctx, cancel := context.WithCancel(context.Background())

	systray.Run(func() {
		systray.SetIcon(iconData)
		mState := systray.AddMenuItem("连接状态", "连接状态")
		mCopy := systray.AddMenuItem("复制socks5地址", "复制socks5地址")
		if config.Version != "" {
			_ = systray.AddMenuItem("软件版本: "+config.Version, "软件版本")
		}
		mQuit := systray.AddMenuItem("退出", "退出")

		updateMenu := func(state gui.State) {
			mState.SetTitle(state.String())
		}

		app.SetStateCallback(func(state gui.State) {
			updateMenu(state)
		})

		updateMenu(app.GetState())

		go func() {
			for {
				select {
				case <-mState.ClickedCh:
					if app.GetState() == gui.StateIdle || app.GetState() == gui.StateDisconnected {
						app.Start(ctx, proxy.Config{
							SSHHost:           cfg.SSH.Host,
							SSHPort:           cfg.SSH.Port,
							SSHUser:           cfg.SSH.User,
							SSHPassword:       cfg.SSH.Password,
							ForceCheckHostKey: cfg.ForceCheckHostKey,
							SOCKS5Listen:      cfg.SOCKS5.ListenAddr,
						})
					}

				case <-mCopy.ClickedCh:
					clipboard.Write(clipboard.FmtText, []byte(socks5URL()))

				case <-mQuit.ClickedCh:
					cancel()
					app.Stop(ctx)
					systray.Quit()
					os.Exit(0)
				}
			}
		}()
	}, func() {
		app.Stop(ctx)
		cancel()
	})

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	<-sigCh
}
