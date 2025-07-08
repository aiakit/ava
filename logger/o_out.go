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

package logger

import (
	"bytes"
	"fmt"
)

type Console struct {
	L Level
}

func (s *Console) Init(string) {
	return
}

func (s *Console) Out(level Level, b *bytes.Buffer) {

	if level < s.L {
		return
	}

	fmt.Println(b.String())

	Buffer.Put(b)
}

func (s *Console) Level() Level {
	return s.L
}

func (s *Console) SetLevel(l Level) {
	s.L = l
}

func (s *Console) Poller() {
	return
}

func (s *Console) Close() {
	return
}

func (s *Console) String() string {
	return "console"
}
