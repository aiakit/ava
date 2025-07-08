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
)

type Outputor interface {
	Init(string)
	Out(level Level, b *bytes.Buffer)
	Level() Level
	SetLevel(level Level)
	Poller()
	Close()
	String() string
}

var DefaultOutput Outputor = &Console{L: DEBUG}
