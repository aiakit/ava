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
	"bytes"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"fmt"
	"io"

	"github.com/oxtoacart/bpool"
)

const (
	//default bytes buffer cap size
	defaultCapSize = 512
	//default packet pool size
	defaultPacketPoolSize = 10240000
	//default bytes buffer cap max size
	//if defaultCapSize > defaultMaxCapSize ? defaultCapSize
	defaultMaxCapSize = 4096
)

// create default pool
var pool = &packetPool{
	capSize:    defaultCapSize,
	maxCapSize: defaultMaxCapSize,
	packets:    make(chan *Packet, defaultPacketPoolSize),
}

type packetPool struct {
	capSize, maxCapSize int
	packets             chan *Packet
}

type Packet struct {
	B *bytes.Buffer
}

func newAvaPacket() *Packet {
	return &Packet{B: bytes.NewBuffer(make([]byte, 0, pool.capSize))}
}

func Recycle(p *Packet) {
	p.B.Reset()

	if p.B.Cap() > pool.maxCapSize {
		p.B = bytes.NewBuffer(make([]byte, 0, pool.maxCapSize))
	}

	select {
	case pool.packets <- p:
	default: //if pool full,throw away
	}
}

func newPacket() (p *Packet) {
	select {
	case p = <-pool.packets:
	default:
		p = newAvaPacket()
	}
	return
}

func payload2avaPacket(b []byte) *Packet {
	r := newPacket()
	r.Write(b)
	return r
}

func payloadIo(body io.ReadCloser) *Packet {
	r := newPacket()
	_, _ = io.Copy(r.B, body)
	return r
}

func payloadBytes(body []byte) *Packet {
	r := newPacket()
	_, _ = io.Copy(r.B, bytes.NewReader(body))
	return r
}

func (r *Packet) recycle() {
	r.B.Reset()
}

func (r *Packet) Len() int {
	return r.B.Len()
}

func (r *Packet) Write(b []byte) {
	r.B.Write(b)
}

func (r *Packet) Bytes() []byte {
	return r.B.Bytes()
}

func (r *Packet) String() string {
	return r.B.String()
}

var (
	defaultErrorPacket = NewErrorPacket()
)

type ErrorPackager interface {
	Error400(c *Context) []byte
	Error500(c *Context) []byte
	Error404(c *Context) []byte
	Error405(c *Context) []byte
}

type ErrorPacket struct{}

func (e *ErrorPacket) Error400(c *Context) []byte {
	p := new(HttpPacket)
	p.Code = 400
	p.Msg = "Bad Request"
	if c == nil {
		return MustMarshal(p)
	}
	return c.Codec().MustEncode(p)
}

func (e *ErrorPacket) Error500(c *Context) []byte {
	p := new(HttpPacket)
	p.Code = 500
	p.Msg = "Internal server error"
	if c == nil {
		return MustMarshal(p)
	}
	return c.Codec().MustEncode(p)
}

func (e *ErrorPacket) Error404(c *Context) []byte {
	p := new(HttpPacket)
	p.Code = 404
	p.Msg = "Not Found"
	if c == nil {
		return MustMarshal(p)
	}
	return c.Codec().MustEncode(p)
}

func (e *ErrorPacket) Error405(c *Context) []byte {
	p := new(HttpPacket)
	p.Code = 405
	p.Msg = "Method Not Allowed"
	if c == nil {
		return MustMarshal(p)
	}
	return c.Codec().MustEncode(p)
}

func NewErrorPacket() *ErrorPacket {
	return &ErrorPacket{}
}

// RsocketRpcVersion rsocket-rpc version
const RsocketRpcVersion = uint16(1)

type metadata struct {
	V  string            `json:"version"`
	S  string            `json:"service"`
	M  string            `json:"method"`
	T  string            `json:"trace"`
	A  string            `json:"address"`
	P  []byte            `json:"payload"`
	Md map[string]string `json:"meta"`
}

func mallocMetadata() *metadata {
	return &metadata{Md: make(map[string]string, 10)}
}

func EncodeMetadata(service, method, tracing string, meta map[string]string) (*metadata, error) {
	b, err := metadataEncode(service, method, tracing, meta)
	if err != nil {
		return nil, err
	}

	return decodeMetadata(b)
}

func (m *metadata) Payload() []byte {
	return m.P
}

func (m *metadata) GetMeta(key string) string {
	return m.Md[key]
}

func (m *metadata) SetMeta(key, value string) {
	m.Md[key] = value
}

func (m *metadata) Service() string {
	return m.S
}

func (m *metadata) Method() string {
	return m.M
}

func (m *metadata) SetMethod(method string) {
	m.M = method
}

func (m *metadata) Version() string {
	return m.V
}

func (m *metadata) Tracing() string {
	return m.T
}

func (m *metadata) Address() string {
	return m.A
}

func (m *metadata) String() string {
	var tr string
	if b := m.Tracing(); len(b) < 1 {
		tr = "<nil>"
	} else {
		tr = "0x" + hex.EncodeToString([]byte(b))
	}

	var s string
	if b := m.Md; len(b) < 1 {
		s = "<nil>"
	} else {
		s = "0x" + hex.EncodeToString(m.getMetadata())
	}
	return fmt.Sprintf(
		"metadata{version=%s, service=%s, method=%s, tracing=%s, metadata=%s}",
		m.Version(),
		m.Service(),
		m.Method(),
		tr,
		s,
	)
}

func (m *metadata) VersionUint16() uint16 {
	return binary.BigEndian.Uint16(m.P)
}

func (m *metadata) getService() string {
	offset := 2

	serviceLen := int(binary.BigEndian.Uint16(m.P[offset : offset+2]))
	offset += 2

	return string(m.P[offset : offset+serviceLen])
}

func (m *metadata) getMethod() string {
	offset := 2

	serviceLen := int(binary.BigEndian.Uint16(m.P[offset : offset+2]))
	offset += 2 + serviceLen

	methodLen := int(binary.BigEndian.Uint16(m.P[offset : offset+2]))
	offset += 2

	return string(m.P[offset : offset+methodLen])
}

func (m *metadata) getTrace() []byte {
	offset := 2

	serviceLen := int(binary.BigEndian.Uint16(m.P[offset : offset+2]))
	offset += 2 + serviceLen

	methodLen := int(binary.BigEndian.Uint16(m.P[offset : offset+2]))
	offset += 2 + methodLen

	tracingLen := int(binary.BigEndian.Uint16(m.P[offset : offset+2]))
	offset += 2

	if tracingLen > 0 {
		return m.P[offset : offset+tracingLen]
	} else {
		return nil
	}
}

func (m *metadata) getMetadata() []byte {
	offset := 2

	serviceLen := int(binary.BigEndian.Uint16(m.P[offset : offset+2]))
	offset += 2 + serviceLen

	methodLen := int(binary.BigEndian.Uint16(m.P[offset : offset+2]))
	offset += 2 + methodLen

	tracingLen := int(binary.BigEndian.Uint16(m.P[offset : offset+2]))
	offset += 2 + tracingLen

	return m.P[offset:]
}

var bb = bpool.NewBufferPool(1024000)

func metadataEncode(service, method, tracing string, metadata map[string]string) (m []byte, err error) {

	w := bb.Get()

	// write version
	err = binary.Write(w, binary.BigEndian, RsocketRpcVersion)
	if err != nil {
		return
	}
	// write service
	err = binary.Write(w, binary.BigEndian, uint16(len(service)))
	if err != nil {
		return
	}
	_, err = w.WriteString(service)
	if err != nil {
		return
	}
	// write method
	err = binary.Write(w, binary.BigEndian, uint16(len(method)))
	if err != nil {
		return
	}
	_, err = w.WriteString(method)
	if err != nil {
		return
	}
	// write tracing
	lenTracing := uint16(len(tracing))
	err = binary.Write(w, binary.BigEndian, lenTracing)
	if err != nil {
		return
	}
	if lenTracing > 0 {
		_, err = w.WriteString(tracing)
		if err != nil {
			return
		}
	}
	// write metadata
	if l := len(metadata); l > 0 {
		_, err = w.Write(MustMarshal(metadata))
		if err != nil {
			return
		}
	}
	m = w.Bytes()

	bb.Put(w)
	return
}

func decodeMetadata(payload []byte) (*metadata, error) {

	m := &metadata{P: payload}

	err := jsonFast.Unmarshal(m.getMetadata(), &m.Md)
	if err != nil {
		Error(err)
		return nil, err
	}

	m.M = m.getMethod()
	if m.M == "" {
		return nil, errors.New("no method")
	}

	m.S = m.getService()

	m.T = BytesToString(m.getTrace())

	m.V = m.GetMeta(defaultHeaderVersion)
	if m.V == "" {
		m.V = defaultVersion
	}

	m.A = m.GetMeta(defaultHeaderAddress)

	return m, nil
}
