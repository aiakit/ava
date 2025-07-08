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
	"errors"
	"math/rand"
	"net"
	"sync"
	"time"
	"unsafe"

	"github.com/google/uuid"
	jsoniter "github.com/json-iterator/go"
	"github.com/rs/xid"
)

func newUUID() string {
	return uuid.New().String()
}

func newXID() string {
	return xid.New().String()
}

var jsonFast = jsoniter.ConfigFastest

func MustMarshal(v interface{}) []byte {
	b, _ := jsonFast.Marshal(v)
	return b
}

func MustUnmarshal(b []byte, v interface{}) {
	err := jsonFast.Unmarshal(b, v)
	if err != nil {
		v = make(map[string]string)
	}
}

func Unmarshal(b []byte, v interface{}) error {
	err := jsonFast.Unmarshal(b, v)
	if err != nil {
		return err
	}

	return nil
}

func MustMarshalString(v interface{}) string {
	b, _ := jsonFast.MarshalToString(v)
	return b
}

func StringToBytes(s string) (b []byte) {
	x := (*[2]uintptr)(unsafe.Pointer(&s))
	h := [3]uintptr{x[0], x[1], x[1]}
	return *(*[]byte)(unsafe.Pointer(&h))
}

func BytesToString(b []byte) string {
	return *(*string)(unsafe.Pointer(&b))
}

var rRand = rand.New(rand.NewSource(time.Now().UnixNano()))

func Rand() *rand.Rand {
	return rRand
}

const letters = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ"

func RandString(length int) string {

	result := make([]byte, length)
	for i := 0; i < length; i++ {
		result[i] = letters[intn(len(letters))]
	}

	return string(result)
}

var randLock sync.Mutex

func intn(n int) int {
	randLock.Lock()
	r := rRand.Intn(n)
	randLock.Unlock()
	return r
}

func RandInt(min, max int) int {
	if min >= max || min == 0 || max == 0 {
		return max
	}
	return rRand.Intn(max-min) + min
}

func RandInt32(min, max int32) int32 {
	if min >= max || min == 0 || max == 0 {
		return max
	}
	return rRand.Int31n(max-min) + min
}

func LocalIp() (string, error) {
	addr, err := net.InterfaceAddrs()
	if err != nil {
		return "", err
	}

	for _, address := range addr {
		if ipNet, ok := address.(*net.IPNet); ok && !ipNet.IP.IsLoopback() {
			if ipNet.IP.To4() != nil {
				return ipNet.IP.String(), nil
			}

		}
	}

	return "", errors.New("cannot find local ip")
}

func RandNumber(digits int) int {
	if digits < 1 {
		return 0
	}
	minNum := 1
	for i := 1; i < digits; i++ {
		minNum *= 10
	}
	maxNum := minNum*10 - 1
	return rRand.Intn(maxNum-minNum+1) + minNum
}
