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
	"github.com/gogo/protobuf/proto"
	jsoniter "github.com/json-iterator/go"
	"github.com/rsocket/rsocket-go/extension"
)

//todo https://github.com/klauspost/compress use compress

var defaultCodec Codecs = jCodec

type Codecs interface {
	Encode(message proto.Message) ([]byte, error)
	Decode(b []byte, message proto.Message) error
	MustEncodeString(message proto.Message) string
	MustEncode(message proto.Message) []byte
	MustDecode(b []byte, message proto.Message)
	Name() string
}

var defaultCodecs = map[string]Codecs{
	extension.ApplicationJSON.String():     jCodec,
	extension.ApplicationProtobuf.String(): &pCodec{},
}

func codecType(contentType string) Codecs {
	c, ok := defaultCodecs[contentType]
	if !ok {
		return defaultCodec
	}
	return c
}

func addCodec(contentType string, c Codecs) {
	defaultCodecs[contentType] = c
}

var jCodec = &jsonCodec{jsonCodec: jsonFast}

//var jCodec = &jsonCodec{jsonCodec: jsoniter.ConfigCompatibleWithStandardLibrary}

type jsonCodec struct {
	jsonCodec jsoniter.API
}

func (j *jsonCodec) Encode(req proto.Message) ([]byte, error) {
	b, err := j.jsonCodec.Marshal(req)
	if err != nil {
		return nil, err
	}

	return b, nil
}

func (j *jsonCodec) MustEncode(req proto.Message) []byte {
	b, _ := j.jsonCodec.Marshal(req)
	return b
}

func (j *jsonCodec) MustEncodeString(req proto.Message) string {
	b, err := j.jsonCodec.MarshalToString(req)
	if err != nil {
		return ""
	}

	return b
}

func (j *jsonCodec) Decode(b []byte, rsp proto.Message) error {
	return j.jsonCodec.Unmarshal(b, rsp)
}

func (j *jsonCodec) MustDecode(b []byte, rsp proto.Message) {
	_ = j.jsonCodec.Unmarshal(b, rsp)
}

func (j *jsonCodec) Name() string {
	return "jsoniter"
}

type pCodec struct{}

func (p *pCodec) Encode(req proto.Message) ([]byte, error) {
	b, err := proto.Marshal(req)
	if err != nil {
		return nil, err
	}

	return b, nil
}

func (p *pCodec) MustEncodeString(req proto.Message) string {
	b, err := proto.Marshal(req)
	if err != nil {
		return ""
	}
	return BytesToString(b)
}

func (p *pCodec) MustEncode(req proto.Message) []byte {
	b, _ := proto.Marshal(req)
	return b
}

func (*pCodec) Decode(b []byte, rsp proto.Message) error {
	err := proto.Unmarshal(b, rsp)
	if err != nil {
		return err
	}
	return nil
}

func (*pCodec) MustDecode(b []byte, rsp proto.Message) {
	proto.Unmarshal(b, rsp)
}

func (*pCodec) Name() string {
	return "gogo_proto"
}
