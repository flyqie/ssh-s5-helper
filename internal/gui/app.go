package gui

import (
	"context"
	"sync"

	"github.com/flyqie/ssh-s5-helper/internal/proxy"
)

type State int

const (
	StateIdle          State = iota
	StateConnecting
	StateConnected
	StateDisconnected
)

func (s State) String() string {
	switch s {
	case StateIdle:
		return "未连接(点击连接)"
	case StateConnecting:
		return "连接中..."
	case StateConnected:
		return "连接成功"
	case StateDisconnected:
		return "连接断开(点击重连)"
	default:
		return "连接异常"
	}
}

type App struct {
	mu    sync.RWMutex
	state State
	addr  string

	server      *proxy.Server
	stateCallback func(State)
}

func New() *App {
	return &App{
		state: StateIdle,
		addr:  "127.0.0.1:1080",
	}
}

func (a *App) SetStateCallback(cb func(State)) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.stateCallback = cb
}

func (a *App) SetListenAddr(addr string) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.addr = addr
}

func (a *App) GetState() State {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.state
}

func (a *App) GetAddr() string {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.addr
}

func (a *App) setState(state State) {
	a.mu.Lock()
	a.state = state
	a.mu.Unlock()
	if a.stateCallback != nil {
		a.stateCallback(state)
	}
}

func (a *App) Start(ctx context.Context, cfg proxy.Config) error {
	a.mu.Lock()
	if a.server != nil && a.server.IsRunning() {
		a.mu.Unlock()
		return nil
	}
	a.state = StateConnecting
	a.mu.Unlock()
	a.stateCallback(StateConnecting)

	server, err := proxy.New(cfg)
	if err != nil {
		a.setState(StateDisconnected)
		return err
	}

	go func() {
		server.Run(ctx)
		server.Close()
		a.setState(StateDisconnected)
	}()

	a.mu.Lock()
	a.server = server
	a.state = StateConnected
	a.mu.Unlock()
	a.stateCallback(StateConnected)

	return nil
}

func (a *App) Stop(ctx context.Context) {
	a.mu.Lock()
	defer a.mu.Unlock()

	if a.server != nil {
		a.server.Close()
		a.server = nil
	}
}