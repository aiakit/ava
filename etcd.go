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
	"context"
	"errors"
	"strings"
	"sync"
	"time"

	"go.etcd.io/etcd/client/v3"
	"go.etcd.io/etcd/client/v3/concurrency"
)

var defaultEtcd *etcd

type etcd struct {
	lock sync.RWMutex

	//etcd client v3
	client *clientv3.Client

	//etcd leaseId
	leaseId clientv3.LeaseID

	//use leaseId to keepalive
	//leaseKeepaliveChan chan *clientv3.LeaseKeepAliveResponse

	//etcd config
	config *clientv3.Config

	//timeout setting
	timeout time.Duration

	//leaseTLL setting
	leaseTLL int64
}

// chaosEtcd init etcd
// if config is nil,use default config setting
func chaosEtcd(timeout time.Duration, leaseTLL int64, config *clientv3.Config) error {
	if defaultEtcd != nil {
		defaultEtcd = nil
	}

	defaultEtcd = new(etcd)

	//defaultEtcd.leaseKeepaliveChan = make(chan *clientv3.LeaseKeepAliveResponse)
	defaultEtcd.config = config
	defaultEtcd.timeout = timeout
	defaultEtcd.leaseTLL = leaseTLL

	if config == nil {
		defaultEtcd.config = &clientv3.Config{
			Endpoints:   []string{"127.0.0.1:2379"},
			DialTimeout: time.Second * 5,
		}
	}

	var err error
	defaultEtcd.client, err = clientv3.New(*defaultEtcd.config)
	if err != nil {
		return err
	}
	return nil
}

// Client get etcd client
func (s *etcd) Client() *clientv3.Client {
	return s.client
}

// PutTimeout put one key/value to etcd with lease setting
func (s *etcd) PutTimeout(key, value string, leaseTLL int64) error {
	ctx, cancel := context.WithTimeout(context.TODO(), s.timeout)
	defer cancel()

	if leaseTLL <= 0 {
		leaseTLL = s.leaseTLL
	}

	rsp, err := clientv3.NewLease(s.client).Grant(ctx, leaseTLL)
	if err != nil {
		return err
	}

	_, err = s.client.Put(context.TODO(), key, value, clientv3.WithLease(rsp.ID))
	return err
}

// PutWithLease put one key/value to etcd with lease setting
// use just one leaseId control all whit lease key/value data
func (s *etcd) PutWithLease(key, value string) error {
	ctx, cancel := context.WithTimeout(context.TODO(), s.timeout)
	defer cancel()

	//if no leaseId,setting lease
	if s.leaseId <= 0 {
		rsp, err := clientv3.NewLease(s.client).Grant(ctx, s.leaseTLL)
		if err != nil {
			return err
		}

		s.leaseId = rsp.ID

		ch, err := s.client.KeepAlive(context.TODO(), rsp.ID)
		if err != nil {
			return err
		}

		go func() {
			for range ch {

			}
		}()
	}

	_, err := s.client.Put(context.TODO(), key, value, clientv3.WithLease(s.leaseId))
	return err
}

// Put one key/value to etcd with no lease setting
func (s *etcd) Put(key, value string) error {
	s.lock.Lock()
	defer s.lock.Unlock()

	_, err := s.client.Put(context.Background(), key, value)
	return err
}

// GetWithLastKey get value with last key
func (s *etcd) GetWithLastKey(key string) ([]byte, error) {

	ctx, cancel := context.WithTimeout(context.Background(), s.timeout)
	defer cancel()

	rsp, err := s.client.Get(ctx, key, clientv3.WithLastKey()...)
	if err != nil {
		return nil, err
	}

	if len(rsp.Kvs) < 1 {
		return nil, errors.New("GetWithLastKey is none by etcd")
	}

	return rsp.Kvs[0].Value, nil
}

// GetWithKey get value with key
func (s *etcd) GetWithKey(key string) ([]byte, error) {

	ctx, cancel := context.WithTimeout(context.Background(), s.timeout)
	defer cancel()

	rsp, err := s.client.Get(ctx, key)
	if err != nil {
		return nil, err
	}

	if len(rsp.Kvs) < 1 {
		return nil, errors.New("GetWithLastKey is none by etcd")
	}

	return rsp.Kvs[0].Value, nil
}

func (s *etcd) GetWithList(key string, opts ...clientv3.OpOption) (map[string][]byte, error) {
	ctx, cancel := context.WithTimeout(context.Background(), s.timeout)
	defer cancel()

	rsp, err := s.client.Get(ctx, key, opts...)
	if err != nil {
		return nil, err
	}

	if rsp.Count < 1 {
		return nil, errors.New("GetWithList is none by etcd")
	}

	var r = make(map[string][]byte, rsp.Count)

	for i := range rsp.Kvs {
		r[BytesToString(rsp.Kvs[i].Key)] = rsp.Kvs[i].Value
	}

	return r, nil
}

func (s *etcd) Delete(key string) error {
	s.lock.Lock()
	defer s.lock.Unlock()

	ctx, cancel := context.WithTimeout(context.TODO(), s.timeout)
	defer cancel()

	_, err := s.client.Delete(ctx, key)

	return err
}

// revoke lease
func (s *etcd) revoke() error {
	var err error
	if s.leaseId > 0 {
		ctx, cancel := context.WithTimeout(context.TODO(), s.timeout)
		defer cancel()
		_, err = s.client.Revoke(ctx, s.leaseId)
	}
	return err
}

func (s *etcd) CloseEtcd() {
	s.lock.Lock()
	defer s.lock.Unlock()

	if s.client != nil {
		if s.leaseId > 0 {
			err := s.revoke()
			if err != nil {
				Error(err)
			}
		}

		err := s.client.Close()
		if err != nil {
			Error(err)
		}
	}
}

type etcdReceive struct {

	//w callback action
	Act WatcherAction

	//callback data
	B map[string][]byte
}

type watch struct {

	//etcd client
	client *clientv3.Client

	//etcd watch channel
	wc clientv3.WatchChan

	//exit cancel
	cancel func()
}

func newEtcdWatch(prefix string, client *clientv3.Client) *watch {
	var w = &watch{
		client: client,
	}

	ctx, cancel := context.WithCancel(context.Background())
	w.wc = client.Watch(ctx, prefix, clientv3.WithPrefix(), clientv3.WithPrevKV())
	w.cancel = cancel
	return w
}

func (w *watch) Watch(prefix string) chan *etcdReceive {
	c := make(chan *etcdReceive)

	go func() {
		for v := range w.wc {
			if v.Err() != nil {
				Error("etcd w err ", v.Err())
				continue
			}

			//w result events
			for _, event := range v.Events {

				if !strings.Contains(BytesToString(event.Kv.Key), prefix) {
					continue
				}

				var (
					a   WatcherAction
					key string
					b   = new(bytes.Buffer)
				)

				switch event.Type {
				case clientv3.EventTypePut:

					a = watcherCreate

					if event.IsModify() {
						a = watcherUpdate
					}

					if event.IsCreate() {
						a = watcherCreate
					}

					key = BytesToString(event.Kv.Key)
					b.Write(event.Kv.Value)

				case clientv3.EventTypeDelete:
					a = watcherDelete

					key = BytesToString(event.PrevKv.Key)
					b.Write(event.PrevKv.Value)

				}

				c <- &etcdReceive{Act: a, B: map[string][]byte{key: b.Bytes()}}
			}
		}
	}()

	return c
}

func (w *watch) CloseWatch() {
	w.cancel()
}

// etcd implementation of service discovery
type etcdRegistry struct {

	//etcd instance
	e *etcd

	//w instance
	w *watch
}

// newRegistry create a new registry with etcd
func newRegistry() *etcdRegistry {
	r := &etcdRegistry{e: defaultEtcd}
	return r
}

// Register register one endpoint to etcd
func (s *etcdRegistry) Register(e *endpoint) error {
	return s.e.PutWithLease(e.Absolute, MustMarshalString(e))
}

// Next return a endpoint
func (s *etcdRegistry) Next(scope string) (*endpoint, error) {

	b, err := s.e.GetWithLastKey(defaultSchema + "/" + scope)
	if err != nil {
		return nil, err
	}

	var e endpoint

	err = jsonFast.Unmarshal(b, &e)
	if err != nil {
		return nil, err
	}

	return &e, nil
}

// List get all endpoint from etcd
func (s *etcdRegistry) List() (services []*endpoint, err error) {
	b, err := s.e.GetWithList(defaultSchema, clientv3.WithPrefix())
	if err != nil {
		return nil, err
	}

	for _, v := range b {
		var e endpoint
		err = jsonFast.Unmarshal(v, &e)
		if err != nil {
			continue
		}
		services = append(services, &e)
	}
	return
}

// Deregister deregister a endpoint ,remove it from etcd
func (s *etcdRegistry) Deregister(e *endpoint) error {
	return s.e.Delete(e.Absolute)
}

func (s *etcdRegistry) Name() string {
	return "etcd"
}

func (s *etcdRegistry) Watch() chan *registryReceive {
	var r = make(chan *registryReceive)

	s.w = newEtcdWatch(defaultSchema, s.e.Client())

	go func() {
		for v := range s.w.Watch(defaultSchema) {
			for key, value := range v.B {
				var e endpoint
				err := jsonFast.Unmarshal(value, &e)
				if err != nil {
					Warnf("action=%s |err=%v", MustMarshalString(v.B), err)
					continue
				}
				r <- &registryReceive{
					Act: v.Act,
					E:   &e,
					Key: key,
				}
			}
		}

		close(r)
	}()

	return r
}

func (s *etcdRegistry) CloseRegistry() {
	if s.w != nil {
		s.w.CloseWatch()
	}
}

var rsyncLockPrefix = "avaRsyncLock/"

func SetDistributedLockPrefix(prefix string) {
	rsyncLockPrefix = prefix
}

// LockMutex is a distributed lock by etcd
// try to lock with a key
// if timeout the lock will be return
// f() is the function what will be lock
// it will return a error
// key is prefix or a unique id
// tll is lock timeout setting
// tryLockTimes is backoff to retry lock.
// ttl is in seconds
func LockMutex(c *Context, key string, ttl int, f func() error) error {

	if ttl <= 0 {
		ttl = 10
	}

	// get a concurrency session
	session, err := concurrency.NewSession(
		defaultEtcd.Client(),
		concurrency.WithContext(c.Context),
		concurrency.WithTTL(ttl))

	if err != nil {
		c.Error(err)
		return err
	}

	defer session.Close()

	mu := concurrency.NewLocker(session, rsyncLockPrefix+key)
	mu.Lock()
	defer mu.Unlock()

	err = f()

	return err
}

var errLock = errors.New("lock failure")

// LockExpire  if lock get failure during time.Second,it will be return lock err
// ttl: lock timeout in seconds
// timeout: if the lock cannot be acquired within the timeout period, an error is returned
func LockExpire(c *Context, key string, ttl int64, f func() error) error {

	if ttl <= 0 {
		ttl = 10
	}

	leaseClient := clientv3.NewLease(defaultEtcd.Client())

	leaseResp, err := leaseClient.Grant(c.Context, ttl)
	if err != nil {
		c.Error(err)
		return err
	}

	resp, err := defaultEtcd.Client().
		Txn(c.Context).
		If(clientv3.Compare(clientv3.Version(rsyncLockPrefix+key), "=", 0)).
		Then(clientv3.OpPut(rsyncLockPrefix+key, "", clientv3.WithLease(leaseResp.ID))).
		Commit()

	if err != nil {
		c.Error(err)
		return err
	}

	if !resp.Succeeded {
		return errLock
	}

	return f()
}
