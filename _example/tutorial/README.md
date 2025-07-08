# simple http api service

```go
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

```

```shell
curl -H "Content-Type:application/json" -X POST -d '{"ping": "ping"}' http://127.0.0.1:10000/hello/http/hi
```