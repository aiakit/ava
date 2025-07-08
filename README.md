# ava

![logo](https://github.com/aiakit/ava/blob/master/ava.jpg)

![GitHub Workflow Status](https://github.com/rsocket/rsocket-go/workflows/Go/badge.svg)
[![Go Report Card](https://goreportcard.com/badge/github.com/aiakit/ava)](https://goreportcard.com/report/github.com/aiakit/ava)
[![Go Reference](https://pkg.go.dev/badge/ava.svg)](https://pkg.go.dev/github.com/aiakit/ava)
![GitHub](https://img.shields.io/github/license/go-ava/ava?logo=rsocket)
![GitHub release (latest SemVer including pre-releases)](https://img.shields.io/github/v/release/go-ava/ava?include_prereleases)

### 👋 ava is a rpc micro framework,it designed with go,and transport protocol by [rsocket-go](https://github.com/rsocket/rsocket-go).

<br>***IT IS UNDER ACTIVE DEVELOPMENT, APIs are unstable and maybe change at any time until release of v1.0.0.***

### 👀 Features

- Simple to use ✨
- Lightweight ✨
- High performance ✨
- Service discovery ✨
- Support websocket and socket same time ✨
- Support json or [gogo proto](https://github.com/gogo/protobuf) when use rpc ✨

### 🌱 Quick start

- first you must install [proto](https://github.com/gogo/protobuf) and [etcd](https://github.com/coreos/etcd)

- install protoc-gen-ava

```shell
    GO111MODULE=on go get github.com/aiakit/ava/cmd/protoc-gen-ava
```

- generate proto file to go file,like [hello.proto](https://gihtub.com/go-ava/ava/_example/tutorials/proto/pbhello.proto)

```shell
    protoc --ava_out = plugins = ava:.*.proto
```

- run a ava service

```go
func main() {
    s := ava.New(
    ava.HttpGetRootPathRedirect("/hello/say/hihttp"),
    ava.Namespace("api.hello"),
    ava.HttpApiAdd("0.0.0.0:10000"),
    ava.TCPApiPort(10001),
    ava.WssApiAddr("0.0.0.0:10002", "/hello"),
    ava.Hijacker(hijack.HijackWriter),
    ava.EtcdConfig(&clientv3.Config{Endpoints: []string{"127.0.0.1:2379"}}),
    )
    
    phello.RegisterSaySrvServer(s.Server(), &hello.Say{})
    
    // for ava/_example/javascript service
    pim.RegisterImServer(s.Server(), im.NewIm())
    
    phello.RegisterHttpServer(s.Server(), &http.Http{})
    
    ipc.InitIpc(s)
    
    s.Run()
}
```

### 💞️ see more [example](https://github.com/aiakit/ava/tree/master/_auxiliary/example) for more help.

### 📫 How to reach me and be a contributor ...

### ✨ TODO ✨

- [ ] topic publish/subscript
- [ ] sidecar
- [ ] more example
- [ ] more singleton tests
- [ ] generate dir
- [ ] command for request service
- [ ] sidecar service
- [ ] config service
- [ ] broker redirect request service
- [ ] logger service
- [ ] simple service GUI



