package ipc

import (
	"github.com/aiakit/ava"
	"github.com/aiakit/ava/_example/tutorial/proto/phello"
	"github.com/aiakit/ava/_example/tutorial/proto/pim"
)

var sayClient phello.SaySrvClient
var imClient pim.ImClient

func InitIpc() {
	sayClient = phello.NewSaySrvClient()
	imClient = pim.NewImClient()
}

func SayStream(c *ava.Context, req *phello.SayReq) chan *phello.SayRsp {
	return sayClient.Stream(c, req, ava.WithName("srv.hello"))
}

func SayChannel(c *ava.Context, req chan *phello.SayReq) chan *phello.SayRsp {
	return sayClient.Channel(c, req, ava.WithName("srv.hello"))
}

func Broadcast(c *ava.Context, req *pim.BroadcastMessage) {
	_, _ = imClient.Broadcast(c, req, ava.WithName("api.hello"), ava.Broadcast())
}
