# GeeRPC

一个轻量级、可扩展的 Go 语言 RPC 框架，实现类似 `net/rpc` 的远程过程调用能力，并支持服务发现与负载均衡。

---

## 项目概览

GeeRPC 是一个自研的 RPC 框架，旨在提供简单易用的远程服务调用能力。它采用自定义二进制协议（魔术号 `0x3bef5c`），支持 TCP 与 HTTP 两种传输方式，内置 Gob 编解码器，并可扩展服务发现、负载均衡、心跳保活等能力，适用于构建分布式微服务应用。

---

## ✨ 特性

- **协议设计**：自定义 RPC 协议，使用魔术号区分请求，支持 Option 协商
- **编解码**：基于 `encoding/gob` 的高效 Gob 编解码器，可扩展 JSON 等格式
- **多传输支持**：支持 TCP 直连与 HTTP CONNECT 两种连接方式
- **反射注册**：通过反射自动发现并注册结构体方法，方法签名：`func (rcvr *T) MethodName(argv T1, reply *T2) error`
- **超时控制**：支持连接超时与请求处理超时
- **服务发现**：支持静态服务列表与基于 Registry 的动态发现
- **负载均衡**：提供随机、轮询两种负载均衡策略
- **广播调用**：可向多个服务实例并发发起调用
- **注册中心**：内置 HTTP 注册中心，支持服务注册与心跳保活

---

## 🚀 快速开始

### 前提条件

| 项目 | 要求 |
|------|------|
| **运行环境** | Go 1.24+ |
| **依赖工具** | 无第三方依赖，仅使用 Go 标准库 |

### 安装步骤

1. **克隆仓库**

```bash
git clone https://github.com/park338/GeeRPC.git
cd GeeRPC
```

2. **安装依赖**

项目无外部依赖，仅需确保 Go 环境正确：

```bash
go mod download
```

3. **配置环境**

无需额外配置文件。可在代码中通过 `GeeRPC.DefaultOption`、`registry.New()` 等接口自定义超时、编解码等参数。

### 基本使用

#### 1. 定义并注册服务

```go
package main

import (
    GeeRPC "codec"
    "log"
    "net"
)

type Foo struct{}

func (f *Foo) Sum(args string, reply *string) error {
    *reply = "Hello, " + args
    return nil
}

func main() {
    l, err := net.Listen("tcp", ":0")
    if err != nil {
        log.Fatal("listen error:", err)
    }
    
    GeeRPC.Register(&Foo{})
    log.Println("RPC server listening on", l.Addr())
    GeeRPC.Accept(l)
}
```

#### 2. TCP 客户端调用

```go
package main

import (
    "codec/client"
    "context"
    "log"
)

func main() {
    client, err := client.Dial("tcp", "localhost:1234")
    if err != nil {
        log.Fatal("dial error:", err)
    }
    defer client.Close()

    var reply string
    ctx := context.Background()
    err = client.Call(ctx, "Foo.Sum", "geerpc", &reply)
    if err != nil {
        log.Fatal("call error:", err)
    }
    log.Println("reply:", reply)
}
```

#### 3. 使用 XDial（支持协议前缀）

```go
// tcp@ 或 http@ 前缀自动选择连接方式
client, err := client.XDial("tcp@localhost:1234")
```

#### 4. 负载均衡与广播

```go
package main

import (
    GeeRPC "codec"
    "codec/xclient"
    "context"
)

func main() {
    d := xclient.NewMultiserversDiscovery([]string{
        "tcp@localhost:8001",
        "tcp@localhost:8002",
    })
    xc := xclient.NewXClient(d, xclient.RandomSelect, GeeRPC.DefaultOption)
    defer xc.Close()

    var reply string
    err := xc.Call(context.Background(), "Foo.Sum", "args", &reply)
    // 或广播到所有实例
    err = xc.Broadcast(context.Background(), "Foo.Sum", "args", &reply)
}
```

#### 5. 运行示例程序

```bash
go run ./main
```

> **说明**：`main/main.go` 为简化演示，需在服务端注册 `Foo` 服务后，客户端调用 `Foo.Sum` 才能正常返回结果。

---

## 📁 项目结构

```
GeeRPC/
├── go.mod              # 模块定义
├── server.go           # RPC 服务端（包 GeeRPC）
├── client/             # RPC 客户端
│   └── client.go
├── codec/              # 编解码
│   ├── codec.go       # Codec 接口与 Header
│   └── gob.go         # Gob 编解码实现
├── xclient/            # 负载均衡客户端
│   ├── xclient.go
│   ├── discovery.go
│   └── discovery_gee.go
├── registry/           # 服务注册中心
│   └── registry.go
└── main/               # 示例入口
    └── main.go
```

---

## 📚 API 速览

| 组件 | 常用 API |
|------|----------|
| 服务端 | `Register(rcvr)`, `Accept(lis)` |
| 客户端 | `Dial(network, addr)`, `DialHTTP()`, `XDial(addr)`, `Call()`, `Go()` |
| 服务发现 | `NewMultiserversDiscovery()`, `NewGeeRegistryDiscovery()` |
| 负载均衡客户端 | `NewXClient()`, `Call()`, `Broadcast()` |
| 注册中心 | `registry.New()`, `HandleHTTP()`, `Heartbeat()` |

---

## 📄 License

MIT
