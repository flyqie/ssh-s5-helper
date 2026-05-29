# SSH SOCKS5 Helper

## 编译

### CLI

```bash
# Windows
go build -o ssh_s5_helper_cli.exe ./cmd/cli

# Linux / macOS
go build -o ssh_s5_helper_cli ./cmd/cli
```

### GUI (托盘)

```bash
# Windows (隐藏控制台窗口)
go build -ldflags "-H windowsgui" -o ssh_s5_helper_gui.exe ./cmd/gui

# Linux / macOS
go build -o ssh_s5_helper_gui ./cmd/gui
```

## 配置

按优先级依次为：

1. 可执行文件同目录的 `config.toml`
2. 环境变量：`SSH_S5_HELPER_HOST`, `SSH_S5_HELPER_PORT`, `SSH_S5_HELPER_USER`, `SSH_S5_HELPER_PASSWORD`, `SSH_S5_HELPER_SOCKS5_LISTEN`
3. 命令行参数

### config.toml 示例

```toml
[SSH]
Host = "your-ssh-host.com"
Port = 22
User = "username"
Password = "password"

[SOCKS5]
ListenAddr = "127.0.0.1:1080"

ForceCheckHostKey = false
```

## 使用

### CLI

```bash
./ssh_s5_helper_cli -H your-ssh-host.com -p 22 -u username -P password
```

### GUI

运行后点击托盘图标即可连接/断开。

## 环境变量

| 变量 | 说明 |
|------|------|
| `SSH_S5_HELPER_HOST` | SSH 服务器地址 |
| `SSH_S5_HELPER_PORT` | SSH 端口 |
| `SSH_S5_HELPER_USER` | 用户名 |
| `SSH_S5_HELPER_PASSWORD` | 密码 |
| `SSH_S5_HELPER_SOCKS5_LISTEN` | SOCKS5 监听地址 |