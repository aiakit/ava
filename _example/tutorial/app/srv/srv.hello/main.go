package main

import (
	"github.com/aiakit/ava"
	"github.com/aiakit/ava/_example/tutorial/proto/phello"

	"github.com/aiakit/ava/_example/tutorial/app/srv/srv.hello/hello"
	"go.etcd.io/etcd/client/v3"
)

func main() {
	ava.SetupService(
		ava.HttpApiAdd("0.0.0.0:10000"),
		ava.Namespace("srv.hello"),
		ava.TCPApiPort(20001),
		ava.EtcdConfig(&clientv3.Config{Endpoints: []string{"127.0.0.1:2379"}}),
	)

	phello.RegisterSaySrvServer(&hello.Say{})

	ava.Run()
}
