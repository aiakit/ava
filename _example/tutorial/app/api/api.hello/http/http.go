package http

import (
	"github.com/aiakit/ava"
	"github.com/aiakit/ava/_example/tutorial/proto/phello"
	"os"
)

type Http struct{}

func (h *Http) Hi(c *ava.Context, req *phello.HttpApiReq, rsp *phello.HttpApiRsp) {
	c.Debug("req", req.Params)
	rsp.Msg = "success"
	rsp.Code = 200
	rsp.Data = "pong"
}

func (h *Http) Upload(c *ava.Context, req *phello.HttpFileReq, rsp *phello.HttpFileRsp) {
	f, err := os.OpenFile(req.FileName, os.O_CREATE|os.O_APPEND|os.O_WRONLY, os.ModePerm)
	if err != nil {
		c.Error(err)
		return
	}

	f.Write(req.Body)

	rsp.Msg = "success"
	rsp.Code = 200
}
