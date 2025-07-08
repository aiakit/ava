// Copyright (c) 2021 ava
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
//  You may obtain a copy of the License at
//
//      https://www.apache.org/licenses/LICENSE-2.0
//
//  Unless required by applicable law or agreed to in writing, software
//  distributed under the License is distributed on an "AS IS" BASIS,
//  WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
//  See the License for the specific language governing permissions and
//  limitations under the License.
//

package ava

import (
	"context"
	"fmt"
	"github.com/aiakit/ava/logger"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/rsocket/rsocket-go/extension"
)

const (
	//default packet pool size
	defaultContextPoolSize = 10240000
)

// create default pool
var gContextPool = &contextPool{
	c: make(chan *Context, defaultContextPoolSize),
}

type contextPool struct {
	c chan *Context
}

func recycleContext(p *Context) {

	p.reset()

	select {
	case gContextPool.c <- p:
	default: //if pool full,throw away
	}
}

func (c *Context) Recycle() {

	c.reset()

	select {
	case gContextPool.c <- c:
	default: //if pool full,throw away
	}
}

func getContext() (p *Context) {
	select {
	case p = <-gContextPool.c:
		p.trace = newSimple()
		p.Metadata = mallocMetadata()
		p.data = make(map[string]interface{}, 10)
		p.Context = context.Background()
	default:
		p = newContext()
	}

	return
}

type Context struct {

	//rpc metadata
	Metadata *metadata

	//trace exists throughout the life cycle of the context
	//trace is request flow trace
	//it's will be from web client,or generated on initialize
	trace Trace

	//Content-Type
	ContentType string

	//http writer
	Writer http.ResponseWriter

	//http request
	Request *http.Request
	//
	////http request body
	//Body io.ReadCloser
	data map[string]interface{}

	isPutFile bool

	RemoteAddr string

	codec Codecs

	context.Context
}

func (c *Context) WithTimeout(timeout time.Duration) context.CancelFunc {
	ctx, cancel := context.WithTimeout(c.Context, timeout)
	c.Context = ctx
	return cancel
}

func Background() *Context {
	return getContext()
}

func (c *Context) WithValue(key string, val interface{}) {
	c.Context = context.WithValue(c.Context, key, val)
}

func (c *Context) Value(key string) interface{} {
	return c.Context.Value(key)
}

func newContext() *Context {
	return &Context{
		trace:    newSimple(),
		Metadata: mallocMetadata(),
		data:     make(map[string]interface{}, 10),
		Context:  context.Background(),
	}
}

func (c *Context) reset() {
	c.Writer = nil
	c.Request = nil
	c.Metadata = nil
	c.codec = nil
	c.ContentType = ""
	c.RemoteAddr = ""
	c.trace = nil
	c.data = nil
	c.isPutFile = false
	c.Context = nil
}

func (c *Context) setCodec() {
	c.codec = codecType(c.ContentType)
}

func (c *Context) Codec() Codecs {
	if c.codec == nil {
		c.codec = codecType(c.ContentType)
	}
	return c.codec
}

func (c *Context) WithMetadata(service, method string, meta map[string]string) (*Context, error) {
	m, err := EncodeMetadata(service, method, c.trace.TraceId(), meta)
	if err != nil {
		return nil, err
	}
	s := *c
	s.Metadata = m

	return &s, nil
}

func (c *Context) Clone() *Context {
	s := *c
	return &s
}

func (c *Context) SetSetupData(value []byte) {
	c.data[defaultHeaderSetup] = value
}

func (c *Context) GetSetupData() []byte {
	b, _ := c.data[defaultHeaderSetup].([]byte)
	return b
}

func fromMetadata(b []byte, dataTYPE, metadataType string) (*Context, error) {

	var m = new(metadata)
	var err error

	switch metadataType {
	case extension.ApplicationJSON.String():
		err = jsonFast.Unmarshal(b, m)
		if err != nil {
			return nil, err
		}
	default:
		m, err = decodeMetadata(b)
		if err != nil {
			return nil, err
		}
	}

	c := getContext()

	c.trace.WithTrace(m.Tracing())
	c.Metadata = m

	c.ContentType = dataTYPE

	if v := m.Md[defaultHeaderContentType]; v != "" {
		c.ContentType = v
	}
	//c.trace.SpreadOnce()
	c.setCodec()

	return c, nil
}

func (c *Context) ClientIP() string {
	clientIP := c.GetHeader("X-Forwarded-For")
	if clientIP != "" {
		s := strings.Split(clientIP, ",")
		if len(s) > 0 {
			clientIP = strings.TrimSpace(s[0])
		}
	}

	if clientIP == "" {
		clientIP = strings.TrimSpace(c.GetHeader("X-Real-Ip"))
	}

	if clientIP != "" {
		return clientIP
	}

	if ip, _, err := net.SplitHostPort(strings.TrimSpace(c.RemoteAddr)); err == nil {
		return ip
	}

	return ""
}

func (c *Context) Get(key string) interface{} {
	return c.data[key]
}

func (c *Context) GetString(key string) string {
	v, ok := c.data[key]
	if ok {
		return v.(string)
	}

	return ""
}

func (c *Context) Set(key string, value interface{}) {
	c.data[key] = value
}

func (c *Context) GetHeader(key string) string {
	return c.Metadata.GetMeta(key)
}

func (c *Context) SetHeader(key, value string) {
	c.Metadata.SetMeta(key, value)
}

func Debug(msg ...interface{}) {
	logger.Debug(fmt.Sprintln(msg...))
}

func Info(msg ...interface{}) {
	logger.Info(fmt.Sprintln(msg...))
}

func Warn(msg ...interface{}) {
	logger.Warn(fmt.Sprintln(msg...))
}

func Error(msg ...interface{}) {
	logger.Error(fmt.Sprintln(msg...))
}

func Fatal(msg ...interface{}) {
	logger.Fatal(fmt.Sprintln(msg...))
}

func Stack(msg ...interface{}) {
	logger.Stack(fmt.Sprintln(msg...))
}

func Debugf(f string, msg ...interface{}) {
	logger.Debug(fmt.Sprintf(f+"\n", msg...))
}

func Infof(f string, msg ...interface{}) {
	logger.Info(fmt.Sprintf(f+"\n", msg...))
}

func Warnf(f string, msg ...interface{}) {
	logger.Warn(fmt.Sprintf(f+"\n", msg...))
}

func Errorf(f string, msg ...interface{}) {
	logger.Error(fmt.Sprintf(f+"\n", msg...))
}

func Fatalf(f string, msg ...interface{}) {
	logger.Fatal(fmt.Sprintf(f+"\n", msg...))
}

func Stackf(f string, msg ...interface{}) {
	logger.Stack(fmt.Sprintf(f+"\n", msg...))
}

func (c *Context) Debug(msg ...interface{}) {
	c.trace.Carrier()
	logger.Debug(c.trace.TraceId() + " |" + fmt.Sprintln(msg...))
}

func (c *Context) Info(msg ...interface{}) {
	c.trace.Carrier()
	logger.Info(c.trace.TraceId() + " |" + fmt.Sprintln(msg...))
}

func (c *Context) Warn(msg ...interface{}) {
	c.trace.Carrier()
	logger.Warn(c.trace.TraceId() + " |" + fmt.Sprintln(msg...))
}

func (c *Context) Error(msg ...interface{}) {
	c.trace.Carrier()
	logger.Error(c.trace.TraceId() + " |" + fmt.Sprintln(msg...))
}

func (c *Context) Debugf(f string, msg ...interface{}) {
	c.trace.Carrier()
	logger.Debug(c.trace.TraceId() + " |" + fmt.Sprintf(f, msg...) + "\n")
}

func (c *Context) Infof(f string, msg ...interface{}) {
	c.trace.Carrier()
	logger.Info(c.trace.TraceId() + " |" + fmt.Sprintf(f, msg...) + "\n")
}

func (c *Context) Warnf(f string, msg ...interface{}) {
	c.trace.Carrier()
	logger.Info(c.trace.TraceId() + " |" + fmt.Sprintf(f, msg...) + "\n")
}

func (c *Context) Errorf(f string, msg ...interface{}) {
	c.trace.Carrier()
	logger.Error(c.trace.TraceId() + " |" + fmt.Sprintf(f, msg...) + "\n")
}
