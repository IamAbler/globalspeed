# GlobalSpeed

## 使用方法

### 桌面端

启动程序，等待自动选择节点后点击 **GO** 测速。可更换节点、调整测速参数、停止测试及查看历史记录。

### CLI

```bash
globalspeed                                  # 自动选择节点并测速
globalspeed nodes                            # 查看节点及 Server ID
globalspeed nodes --province 江苏 --operator 电信
globalspeed -s 1503                          # 指定节点
globalspeed --duration 5 --connections 2 --max-mib 64
globalspeed -f json                          # JSON 输出
globalspeed --progress no                    # 关闭实时进度
globalspeed --no-history                     # 不保存本次历史
globalspeed update                           # 更新节点目录
globalspeed nodes --no-update                # 使用本地目录
globalspeed select --province 江苏 --operator 电信
globalspeed history                          # 查看历史记录
globalspeed dns example.com                  # 查询 DNS
globalspeed --help                           # 查看帮助
```

Windows 使用 `globalspeed.exe`。按 Ctrl+C 停止测速。

## 编译方法

### CLI

需要 Go 1.25+。

```bash
go build -o globalspeed ./cmd/globalspeed
```

编译 Windows、Linux、macOS 的 amd64 / arm64 版本，需要 Python 3：

```bash
python3 scripts/build_cli.py
```

输出目录：`build/bin/`。

### 桌面端

需要 Go 1.25+、Node.js 22+ 和对应平台的依赖：

| 平台 | 依赖 |
| --- | --- |
| Windows | WebView2 Runtime |
| macOS | Xcode Command Line Tools |
| Linux | C 编译器、GTK 3、WebKitGTK 4.1、pkg-config |

```bash
go install github.com/wailsapp/wails/v2/cmd/wails@v2.15.0
npm --prefix frontend ci
```

在对应系统上编译：

```bash
# Windows
wails build -tags desktop

# macOS（Intel / Apple Silicon）
wails build -tags desktop -platform darwin/universal

# Ubuntu / Debian
sudo apt-get install build-essential pkg-config libgtk-3-dev libwebkit2gtk-4.1-dev iputils-ping
wails build -tags desktop,webkit2_41
```

输出目录：`build/bin/`。开发运行使用 `wails dev -tags desktop`；Linux 加上 `webkit2_41` 标签。

## 默认存储位置

桌面端与 CLI 共用以下目录：

| 系统 | 目录 |
| --- | --- |
| Windows | `%APPDATA%\globalspeed\` |
| macOS | `~/Library/Application Support/globalspeed/` |
| Linux | `$XDG_CONFIG_HOME/globalspeed/`，未设置时为 `~/.config/globalspeed/` |

- `history.json`：测速历史。
- `serverlist.json`：加密节点目录。

查询实际路径：

```bash
globalspeed history --path
globalspeed nodes --path
```
