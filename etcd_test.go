package ava

import (
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestMock(t *testing.T) {

	err := defaultEtcd.Put("/test/ava", "data")
	if err != nil {
		t.Fatal(err)
	}
	v, err := defaultEtcd.GetWithKey("/test/ava")
	if err != nil {
		t.Fatal(err)
	}

	assert.Equal(t, string(v), "data")
}

func TestLockMutex(t *testing.T) {

	const key = "test"
	var ch = make(chan string)
	go func() {
		c := Background()
		cancel := c.WithTimeout(time.Second * 5)
		defer cancel()
		err := LockMutex(
			c, key, 5, func() error {
				time.Sleep(time.Second * 4)
				ch <- "1"
				fmt.Println("run first!")
				return nil
			},
		)

		if err != nil {
			panic(err)
		}
	}()

	go func() {
		time.Sleep(time.Second)
		c := Background()
		cancel := c.WithTimeout(time.Second)
		defer cancel()
		err := LockMutex(
			c, key, 5, func() error {
				ch <- "2"
				fmt.Println("run second!")
				return nil
			},
		)

		if err != nil {
			panic(err)
		}
	}()

	assert.Equal(t, <-ch, "1")
	assert.Equal(t, <-ch, "2")
}

func TestLockExpire(t *testing.T) {

	const key = "test"
	var ch = make(chan string)
	go func() {
		c := Background()
		cancel := c.WithTimeout(time.Second * 2)
		defer cancel()
		err := LockExpire(
			c, key, 2, func() error {
				//time.Sleep(time.Second * 4)
				ch <- "1"
				fmt.Println("run first!")
				return nil
			},
		)

		if err != nil {
			panic(err)
		}
	}()

	go func() {
		time.Sleep(time.Second * 3)
		c := Background()
		cancel := c.WithTimeout(time.Second * 5)
		defer cancel()
		err := LockExpire(
			c, key, 5, func() error {
				//time.Sleep(time.Second * 4)
				ch <- "2"
				fmt.Println("run second!")
				return nil
			},
		)

		if err != nil {
			panic(err)
		}
	}()

	fmt.Println(<-ch)
	fmt.Println(<-ch)
}
