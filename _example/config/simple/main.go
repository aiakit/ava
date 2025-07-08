package main

import (
	"github.com/aiakit/ava"

	"go.etcd.io/etcd/client/v3"
)

func main() {

	//new config use default option
	err := ava.NewConfig(ava.SetEtcdConfig(&clientv3.Config{Endpoints: []string{"127.0.0.1:2379"}}))
	if err != nil {
		panic(err)
	}

	const key = "test"
	var result struct {
		Name string `json:"name"`
		Age  int    `json:"age"`
	}

	coverPublic(key, &result)
	coverPrivate(key, &result)
}

// put key/value to etcd:
//
//go:generate etcdctl put  configava/v1.0.0/public/ava.test "{ \"name\":\"ava\", \"age\":18 }"
func coverPublic(key string, v interface{}) {
	//simple public use
	//the key is ava.test
	err := ava.ConfigDecPublic(key, v)
	if err != nil {
		panic(err)
	}

	//output: ------ {ava 18}
}

// put key/value to etcd:
//
//go:generate etcdctl put  configava/v1.0.0/private/test "{ \"name\":\"ava\", \"age\":18 }"
func coverPrivate(key string, v interface{}) {
	//the key is test
	err := ava.ConfigDecPrivate(key, v)
	if err != nil {
		panic(err)
	}

	//output: ------ {ava 18}
}
