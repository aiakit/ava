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
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

const fileNameFormat = "2006.01.02.15:04:05.000"

const (
	B = 1 << (10 * iota)
	KB
	MB
	GB
	TB
	PB
)

const (
	DefaultFileMaxDelta = 100
	DefaultBufferSize   = 512 * KB
	DefaultFileMaxSize  = 256 * MB
)

type fileOption struct {
	link          string
	name          string // project name
	fileName      string
	path          string
	async         bool
	prefix        string
	maxFileSize   int
	maxBufferSize int
	maxBucketSize int
	rotate        bool
	interval      time.Duration
	zone          *time.Location
	modePerm      int
}

type fileOptions func(*fileOption)

func newFileOpts(opts ...fileOptions) fileOption {
	opt := fileOption{}

	for i := range opts {
		opts[i](&opt)
	}

	if opt.name == "" {
		opt.name = "ava"
	}

	if opt.path == "" {
		opt.path = "./logs"
	}

	opt.link = opt.path + string(os.PathSeparator) + opt.name + ".log"

	if opt.maxFileSize == 0 {
		opt.maxFileSize = DefaultFileMaxSize
	}

	opt.maxFileSize -= DefaultFileMaxDelta

	if opt.maxBufferSize == 0 {
		opt.maxBufferSize = DefaultBufferSize
	}

	if opt.interval == 0 {
		opt.interval = time.Millisecond * 500
	}

	if opt.zone == nil {
		opt.zone = time.Local
	}

	if opt.prefix == "" {
		opt.prefix = ""
	}

	if opt.modePerm == 0 {
		opt.modePerm = int(os.ModePerm)
	}

	opt.link = filepath.Join(opt.path, opt.name+".log")
	return opt
}

func Name(name string) fileOptions {
	return func(fileOption *fileOption) {
		fileOption.name = name
	}
}

func Interval(interval time.Duration) fileOptions {
	return func(fileOption *fileOption) {
		fileOption.interval = interval
	}
}

func Link(link string) fileOptions {
	return func(fileOption *fileOption) {
		fileOption.link = link
	}
}

func Path(p string) fileOptions {
	return func(fileOption *fileOption) {
		fileOption.path = p
	}
}

func Async() fileOptions {
	return func(fileOption *fileOption) {
		fileOption.async = true
	}
}

func Prefix(prefix string) fileOptions {
	return func(fileOption *fileOption) {
		fileOption.prefix = prefix
	}
}

func MaxFileSize(maxFileSize int) fileOptions {
	return func(fileOption *fileOption) {
		fileOption.maxBufferSize = maxFileSize
	}
}

func MaxBufferSize(maxBufferSize int) fileOptions {
	return func(fileOption *fileOption) {
		fileOption.maxBufferSize = maxBufferSize
	}
}

func Rotate() fileOptions {
	return func(fileOption *fileOption) {
		fileOption.rotate = true
	}
}

func Zone(zone *time.Location) fileOptions {
	return func(fileOption *fileOption) {
		fileOption.zone = zone
	}
}

func FileModel(perm int) fileOptions {
	return func(fileOption *fileOption) {
		fileOption.modePerm = perm
	}
}

type FileSysOut struct {
	sync.Mutex
	opts       fileOption
	file       *os.File
	fileWriter *bufio.Writer
	fileSize   int
	async      bool
	Bucket     chan *bytes.Buffer
	close      chan struct{}
	level      Level
	timestamp  int
	ticker     *time.Ticker
}

func NewFs(opts ...fileOptions) *FileSysOut {
	f := &FileSysOut{}
	f.opts = newFileOpts(opts...)
	return f
}

func (l *FileSysOut) Init(string) {
	return
}

func (l *FileSysOut) Out(level Level, b *bytes.Buffer) {
	if level < l.level {
		return
	}

	l.Bucket <- b
}

func (l *FileSysOut) Level() Level {
	return l.level
}

func (l *FileSysOut) SetLevel(level Level) {
	l.level = level
}

func (l *FileSysOut) String() string {
	return "file"
}

func (l *FileSysOut) Link() string {
	return l.opts.link
}

func (l *FileSysOut) Name() string {
	return l.opts.name
}

func (l *FileSysOut) Path() string {
	return l.opts.path
}

func (l *FileSysOut) Async() bool {
	return l.opts.async
}

func (l *FileSysOut) Prefix() string {
	return l.opts.prefix
}

func (l *FileSysOut) MaxBucketSize() int {
	return l.opts.maxBucketSize
}

func (l *FileSysOut) Rotate() bool {
	return l.opts.rotate
}

func (l *FileSysOut) Interval() time.Duration {
	return l.opts.interval
}

func (l *FileSysOut) Filename() string {
	return l.opts.fileName
}

func (l *FileSysOut) Zone() *time.Location {
	return l.opts.zone
}

func (l *FileSysOut) FileModel() os.FileMode {
	return os.FileMode(l.opts.modePerm)
}

func (l *FileSysOut) MaxBufferSize() int {
	return l.opts.maxBufferSize
}

func (l *FileSysOut) loadLink() (err error) {
	l.Lock()
	defer l.Unlock()

	l.opts.fileName, err = isLink(l.Link())
	if err != nil {
		return err
	}

	l.file, err = open(l.Filename(), l.FileModel())
	if err != nil {
		return err
	}

	info, err := os.Stat(l.Filename())
	if err != nil {
		return err
	}

	t, err := time.ParseInLocation(
		fileNameFormat,
		getFilenamePrefix(info.Name()),
		l.Zone(),
	)

	if err != nil {
		return err
	}

	l.timestamp = convertTime(t)
	l.fileSize = int(info.Size())
	l.fileWriter = bufio.NewWriterSize(l.file, l.MaxBufferSize())

	return nil
}

func (l *FileSysOut) create() error {
	l.Lock()
	defer l.Unlock()

	var err error
	if !pathIsExist(l.Path()) {
		if err = os.MkdirAll(l.Path(), os.ModePerm); err != nil {
			return err
		}
	}

	var now = time.Now()

	l.timestamp = convertTime(now)

	l.opts.fileName = filepath.Join(
		l.Path(),
		now.Format(fileNameFormat)+".log",
	)

	f, err := open(l.Filename(), l.FileModel())
	if err != nil {
		return err
	}

	l.file = f
	l.fileWriter = bufio.NewWriterSize(f, l.MaxBucketSize())

	_ = os.Remove(l.Link())
	return os.Symlink(l.Filename(), l.Link())
}

func (l *FileSysOut) Poller() {
	if l.loadLink() != nil {
		err := l.create()
		if err != nil {
			panic(err)
		}
	}

QUIT:
	for {
		select {
		case <-l.ticker.C:
			if l.fileWriter.Size() > 0 {
				l.fflush()
			}

		case n := <-l.Bucket:
			l.rotateWrite(n)

		case <-l.close:
			break QUIT
		}
	}
}

func convertTime(t time.Time) int {
	y, m, d := t.Date()
	return y*10000 + int(m)*100 + d*1
}

func (l *FileSysOut) fflush() {
	l.Lock()
	_ = l.fileWriter.Flush()
	l.Unlock()
}

func (l *FileSysOut) rotateWrite(b *bytes.Buffer) {
	n, _ := l.fileWriter.Write(b.Bytes())
	l.fileSize += n

	Buffer.Put(b)

	if n <= 0 {
		return
	}

	timestamp := convertTime(time.Now())

	if l.fileSize <= l.opts.maxFileSize && timestamp <= l.timestamp {
		return
	}

	l.Lock()
	defer l.Unlock()

	l.fflush()
	l.closeFile()
	err := l.create()
	if err != nil {
		fmt.Println("rotateWrite and create file err=", err)
		return
	}
	l.fileWriter.Reset(l.file)
}

func (l *FileSysOut) closeFile() {
	l.Lock()
	defer l.Unlock()

	if l.file != nil {
		_ = l.file.Close()
	}
}

func (l *FileSysOut) Close() {
	l.Lock()
	defer l.Unlock()

	if l.ticker != nil {
		l.ticker.Stop()
	}

	l.fflush()
	l.closeFile()

	close(l.Bucket)
	l.close <- struct{}{}
}

func open(name string, perm os.FileMode) (*os.File, error) {
	return os.OpenFile(name, os.O_WRONLY|os.O_CREATE|os.O_APPEND|os.O_SYNC, perm)
}

func isLink(filename string) (string, error) {
	fi, err := os.Lstat(filename)
	if err != nil {
		return "", err
	}

	if fi.Mode()&os.ModeSymlink != 0 {
		name, err := os.Readlink(filename)
		if err != nil {
			return "", err
		}
		return name, nil
	}

	return "", errors.New("not symlink")
}

func pathIsExist(path string) bool {
	_, err := os.Stat(path)
	if err != nil {
		return os.IsExist(err)
	}
	return true
}

func getFilenamePrefix(s string) string {
	return strings.TrimSuffix(path.Base(s), ".log")
}
