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
	"github.com/oxtoacart/bpool"
	"path"
	"runtime"
	"strconv"
	"time"
)

func init() {
	// Call(4) is the actual line where used
	//Overload(Call(-1))
	Overload(Call(4))
}

type Level uint8

const (
	NONE Level = 0x00 + iota
	DEBUG
	STACK
	INFO
	WARN
	ERR
	FATAL
)

type Config = string

type Detail struct {
	Name      string `json:"name"`
	Line      string `json:"line"`
	Prefix    string `json:"prefix"`
	Trace     uint64 `json:"trace"`
	Content   string `json:"content"`
	Level     string `json:"level"`
	Timestamp string `json:"timestamp"`
}

var Buffer = bpool.NewBufferPool(10240000)

type Option struct {
	//set runtime caller deep
	call int

	//set soft link file name
	name string

	//set log content prefix
	prefix string

	//format log content
	format Formatter

	//out for file or std,default is stdout
	out Outputor
}

type Options func(*Option)

func newOpts(opts ...Options) Option {
	opt := Option{}

	for i := range opts {
		opts[i](&opt)
	}

	if opt.format == nil {
		opt.format = defaultFormat
	}

	if opt.name == "" {
		opt.name = "ava"
	}

	if opt.out == nil {
		opt.out = DefaultOutput
	}

	if opt.prefix == "" {
		opt.prefix = ""
	}

	if opt.call == 0 {
		opt.call = 4
	}

	return opt
}

func Call(call int) Options {
	return func(option *Option) {
		option.call = call
	}
}

func Output(out Outputor) Options {
	return func(option *Option) {
		option.out = out
	}
}

func SetInfo() {
	defaultLogger.Output().SetLevel(INFO)
}

func LinkFile(name string) Options {
	return func(option *Option) {
		option.name = name
	}
}

func ContentPrefix(prefix string) Options {
	return func(option *Option) {
		option.prefix = prefix
	}
}

func Format(format Formatter) Options {
	return func(option *Option) {
		option.format = format
	}
}

var defaultLogger *log

type log struct {
	opts   Option
	detail *Detail
}

func Overload(opts ...Options) {
	if defaultLogger != nil {
		defaultLogger = nil
	}

	defaultLogger = &log{opts: newOpts(opts...)}

	defaultLogger.detail = &Detail{
		Name:   defaultLogger.Name(),
		Prefix: defaultLogger.Prefix(),
	}
}

func (l *log) Fire(level, msg string) *Detail {
	d := *l.detail
	if l.opts.call >= 0 {
		d.Line = l.caller()
	}
	d.Timestamp = time.Now().Format(l.Formatter().Layout())
	d.Level = level
	d.Content = msg
	return &d
}

func (l *log) Name() string {
	return l.opts.name
}

func (l *log) Prefix() string {
	return l.opts.prefix
}

func (l *log) Formatter() Formatter {
	return l.opts.format
}

func (l *log) Output() Outputor {
	return l.opts.out
}

func (l *log) caller() string {
	_, file, line, ok := runtime.Caller(l.opts.call)
	//funcName := runtime.FuncForPC(pc).SceneName()
	if !ok {
		file = "???"
		line = 0
	}
	return path.Base(file) + ":" + strconv.Itoa(line)
}

func Close() {
	defaultLogger.opts.out.Close()
}

func Debug(content string) {
	b := defaultLogger.
		Formatter().
		Format(defaultLogger.Fire("DBUG", content))

	defaultLogger.
		Output().
		Out(DEBUG, b)
}

func Info(content string) {
	b := defaultLogger.
		Formatter().
		Format(defaultLogger.Fire("INFO", content))

	defaultLogger.
		Output().
		Out(INFO, b)
}

func Warn(content string) {
	b := defaultLogger.
		Formatter().
		Format(defaultLogger.Fire("WARN", content))

	defaultLogger.
		Output().
		Out(WARN, b)
}

func Error(content string) {
	b := defaultLogger.
		Formatter().
		Format(defaultLogger.Fire("ERRO", content))

	defaultLogger.
		Output().
		Out(ERR, b)
}

func Fatal(content string) {
	b := defaultLogger.
		Formatter().
		Format(defaultLogger.Fire("FATA", content))

	defaultLogger.
		Output().
		Out(FATAL, b)
}

func Stack(content string) {

	buf := make([]byte, 1<<20)
	n := runtime.Stack(buf, true)
	content += string(buf[:n]) + "\n"

	b := defaultLogger.
		Formatter().
		Format(defaultLogger.Fire("STAK", content))

	defaultLogger.
		Output().
		Out(STACK, b)
}
