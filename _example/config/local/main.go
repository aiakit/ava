package main

import (
	"fmt"
	"github.com/aiakit/ava"

	"go.etcd.io/etcd/client/v3"
)

func main() {

	//new config use default option
	err := ava.NewConfig(ava.LocalFile(), ava.SetEtcdConfig(&clientv3.Config{
		Endpoints: []string{"127.0.0.1:2379"},
	}))
	if err != nil {
		panic(err)
	}

	const key = "test"
	type data struct {
		Name string `json:"name"`
		Age  int    `json:"age"`
	}

	err = ava.ConfigPutPrivate(key, ava.MustMarshalString(&data{
		Name: "ava",
		Age:  18,
	}))
	if err != nil {
		panic(err)
	}

	var d data

	err = ava.ConfigDecPrivate(key, &d)
	if err != nil {
		panic(err)
	}

	fmt.Println("--------", d)
}
