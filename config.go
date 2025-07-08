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
	"fmt"
	"io"
	"os"
	"strings"
	"sync"
	"time"

	"go.etcd.io/etcd/client/v3"
)

var defaultConfigSchema = "configava"

type configOption struct {

	//etcd
	e *etcd

	//disableDynamic switch
	disableDynamic bool

	//config schema on etcd
	//schema usually is public config dir
	schema string

	//private config on etcd
	//cannot user private to cover public key,because it's will be inconsistent when
	//modify public key when modify private same key like ava.test
	private string

	//public config on etcd
	public string

	//publicPrefix eg. ava.test "ava." is prefix
	publicPrefix string

	//config version
	version string

	//backup file path
	backupPath string

	//localFile switch
	//if true,will load local config.json to
	localFile bool

	//switch to output config to log
	logOut bool

	//etcd config
	etcdConfig *clientv3.Config

	//f will be run after config already setup
	f []func() error
}

type configOptions func(option *configOption)

// Chaos is config already setup and do Chaos functions
func Chaos(f ...func() error) configOptions {
	return func(option *configOption) {
		option.f = f
	}
}

func Private(private string) configOptions {
	return func(option *configOption) {
		option.private = private
	}
}

func Public(public string) configOptions {
	return func(option *configOption) {
		option.public = public
	}
}

func LocalFile() configOptions {
	return func(option *configOption) {
		option.localFile = true
	}
}

func LogOut() configOptions {
	return func(option *configOption) {
		option.logOut = true
	}
}

func DisableDynamic() configOptions {
	return func(option *configOption) {
		option.disableDynamic = true
	}
}

func ConfigSchema(schema string) configOptions {
	return func(option *configOption) {
		option.schema = schema
	}
}

func Version(version string) configOptions {
	return func(option *configOption) {
		option.version = version
	}
}

func Backup(path string) configOptions {
	return func(option *configOption) {
		option.backupPath = path
	}
}

func Prefix(prefix string) configOptions {
	return func(option *configOption) {
		option.publicPrefix = prefix
	}
}

func SetEtcdConfig(config *clientv3.Config) configOptions {
	return func(option *configOption) {
		option.etcdConfig = config
	}
}

func newConfigOpts(opts ...configOptions) configOption {
	opt := configOption{}

	for i := range opts {
		opts[i](&opt)
	}

	if opt.schema == "" {
		opt.schema = defaultConfigSchema
	}

	if opt.version == "" {
		opt.version = defaultVersion
	}

	opt.schema += "/" + opt.version

	if opt.backupPath == "" {
		opt.backupPath = "./config.json"
	}

	if opt.private == "" {
		opt.private = "private"
	}

	opt.private = opt.schema + "/" + opt.private + "/"

	if opt.public == "" {
		opt.public = "public"
	}

	opt.public = opt.schema + "/" + opt.public + "/"

	opt.e = defaultEtcd

	if opt.publicPrefix == "" {
		opt.publicPrefix = "ava."
	}

	if strings.Contains(opt.private, opt.publicPrefix) {
		panic("private cannot contains public prefix")
	}

	return opt
}

// Configuration Center
// use etcd,
var gRConfig *config

type config struct {

	//config option
	opt configOption

	lock sync.RWMutex

	//config data local cache
	data map[string][]byte

	//close signal
	close chan struct{}

	//receive etcd callback data
	action chan *etcdReceive

	//w etcd changed
	w *watch

	//cache the config objects
	//when create or update action occur
	//them will be updated
	cache map[string]interface{}
}

func NewConfig(opts ...configOptions) error {
	gRConfig = &config{
		opt:   newConfigOpts(opts...),
		data:  make(map[string][]byte),
		cache: make(map[string]interface{}),
		close: make(chan struct{}),
	}

	if gRConfig.opt.e == nil && gRConfig.opt.etcdConfig == nil {
		panic("config etcd is nil,you musa set etcdConfig settings for config!")
	}

	if defaultEtcd != nil && gRConfig.opt.etcdConfig != nil {
		panic("service already set etcd config,don't set repeat settings!")
	}

	if gRConfig.opt.e == nil && gRConfig.opt.etcdConfig != nil {
		// init e.DefaultEtcd
		err := chaosEtcd(time.Second*5, 60, gRConfig.opt.etcdConfig)
		if err != nil {
			panic("etcdConfig occur error: " + err.Error())
		}
		gRConfig.opt.e = defaultEtcd
	}

	if !gRConfig.opt.disableDynamic {
		gRConfig.w = newEtcdWatch(gRConfig.opt.schema, gRConfig.opt.e.Client())
		gRConfig.action = gRConfig.w.Watch(gRConfig.opt.schema)
	}

	if gRConfig.opt.localFile {
		err := gRConfig.loadLocalFile()
		if err != nil {
			Error(err)
		}
	} else {
		err := gRConfig.configListAndSync()
		if err != nil {
			Error(err)
			return err
		}
	}

	if !gRConfig.opt.disableDynamic {
		go gRConfig.update()
	}

	if len(gRConfig.opt.f) > 0 {
		for i := range gRConfig.opt.f {
			err := gRConfig.opt.f[i]()
			if err != nil {
				return err
			}
		}
	}

	return nil
}

func (c *config) configListAndSync() error {
	c.lock.Lock()
	defer c.lock.Unlock()

	publicData, err := c.opt.e.GetWithList(c.opt.public, clientv3.WithPrefix())
	if err == nil {
		for k, v := range publicData {
			c.data[getFsName(k)] = v
		}
	}

	privateData, err := c.opt.e.GetWithList(c.opt.private, clientv3.WithPrefix())
	if err == nil {
		for k, v := range privateData {
			c.data[getFsName(k)] = v
		}
	}

	return c.backup()
}

// ConfigPutPublic put public key value to etcd and cache data
func ConfigPutPublic(key, value string) error {
	c := gRConfig
	c.lock.Lock()
	defer c.lock.Unlock()

	err := c.opt.e.Put(c.opt.public+c.opt.publicPrefix+key, value)
	if err != nil {
		return err
	}
	c.data[c.opt.publicPrefix+key] = []byte(value)

	return nil
}

// ConfigPutPrivate put private key value to etcd and cache data
func ConfigPutPrivate(key, value string) error {
	c := gRConfig
	c.lock.Lock()
	defer c.lock.Unlock()

	err := c.opt.e.Put(c.opt.private+key, value)
	if err != nil {
		return err
	}

	c.data[key] = []byte(value)

	return nil
}

func (c *config) backup() error {
	fs, err := os.OpenFile(c.opt.backupPath, os.O_CREATE|os.O_RDWR|os.O_TRUNC, os.ModePerm)

	if err != nil {
		return err
	}

	var data = make(map[string]interface{})
	for k, v := range c.data {
		var tmp = make(map[string]interface{})
		err = jsonFast.Unmarshal(v, &tmp)
		if err != nil {
			continue
		}
		data[k] = tmp
	}

	b, err := jsonFast.Marshal(data)
	if err != nil {
		return err
	}

	_, _ = fs.Write(b)

	_ = fs.Close()

	return nil
}

// loadLocalFile load local config file to etcd
func (c *config) loadLocalFile() error {
	c.lock.Lock()
	defer c.lock.Unlock()

	fs, err := os.Open(c.opt.backupPath)
	if err != nil {
		return err
	}
	fd, err := io.ReadAll(fs)
	if err != nil {
		return err
	}

	if gRConfig.opt.logOut {
		Infof("loadLocalFile |data=%s", string(fd))
	}

	var tmp = make(map[string]interface{})
	err = jsonFast.Unmarshal(fd, &tmp)
	if err != nil {
		return err
	}

	for i := range tmp {
		c.data[i] = MustMarshal(tmp[i])
	}

	return nil
}

func getFsName(s string) string {
	array := strings.Split(s, "/")

	if len(array) > 0 {
		s = array[len(array)-1]
	}

	return s
}

func (c *config) update() {
	if !c.opt.disableDynamic {
		for {
			select {
			case data := <-c.action:
				// sync config all
				c.lock.Lock()

				switch data.Act {
				case watcherCreate, watcherUpdate:
					for k, v := range data.B {

						var key = getFsName(k)

						c.data[key] = v

						//load create config or update config to exist object
						if f, ok := c.cache[key]; ok { //if ok,load to object
							var err error
							if strings.Contains(key, c.opt.publicPrefix) {
								err = ConfigDecPublic(key, f)
							} else {
								err = ConfigDecPrivate(key, f)
							}

							if err != nil {
								Error(err)
							}
						}
					}

				case watcherDelete:
					for k := range data.B {

						var key = getFsName(k)
						if _, ok := c.data[key]; ok {
							delete(c.data, key)
							delete(c.cache, key)
						}
					}
				}

				c.lock.Unlock()

			case <-c.close:
				return
			}
		}
	}
}

func configClose() {
	gRConfig.lock.Lock()
	gRConfig.data = nil
	gRConfig.lock.Unlock()
	gRConfig.close <- struct{}{}
}

// ConfigDecPublic decode data to config and config will be updated when etcd w change.
func ConfigDecPublic(key string, v interface{}) error {

	d, ok := gRConfig.data[gRConfig.opt.publicPrefix+key]
	if !ok {
		return fmt.Errorf("config: %s not found", key)
	}
	err := jsonFast.Unmarshal(d, v)
	if err != nil {
		return err
	}

	gRConfig.cache[key] = v

	if gRConfig.opt.logOut {
		Infof("ConfigDecPublic |key=%s |value=%s", key, MustMarshalString(v))
	}

	return nil
}

// ConfigDecPrivate decode data to config and config will be updated when etcd w change.
func ConfigDecPrivate(key string, v interface{}) error {

	d, ok := gRConfig.data[key]
	if !ok {
		return fmt.Errorf("config: %s not found", key)
	}

	err := jsonFast.Unmarshal(d, v)
	if err != nil {
		return err
	}

	gRConfig.cache[key] = v

	if gRConfig.opt.logOut {
		Infof("ConfigDecPrivate |key=%s |value=%s", key, MustMarshalString(v))
	}

	return nil
}
