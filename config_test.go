package ava

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

type configData struct {
	Name string
	Age  int
}

func setup() {

	err := NewConfig(
		Public("public"),
		Private("private"),
		DisableDynamic(),
		LogOut(),
		Prefix("ava."),
	)
	if err != nil {
		panic(err)
	}
}

func TestDecode(t *testing.T) {
	setup()

	{

		var err error

		data := `{"name":"ava","age":1}`
		key := "test"

		//key must equal
		assert.Equal(t, "configava/v1.0.0/public/ava.test", gRConfig.opt.public+gRConfig.opt.publicPrefix+key)

		err = ConfigPutPublic(key, data)
		if err != nil {
			t.Fatal(err)
		}

		var d configData
		err = ConfigDecPublic(key, &d)
		if err != nil {
			t.Fatal(err)
		}

		assert.Equal(t, d.Name, "ava")
		assert.Equal(t, d.Age, 1)

		var d1 configData
		err = ConfigDecPrivate(key, &d1)
		assert.NotNil(t, err)
		assert.Equal(t, d1.Name, "")
		assert.Equal(t, d1.Age, 0)
	}

	{
		var err error

		data := `{"name":"ava","age":1}`
		key := "test"

		//key must equal
		assert.Equal(t, "configava/v1.0.0/private/test", gRConfig.opt.private+key)

		err = ConfigPutPrivate(key, data)
		if err != nil {
			t.Fatal(err)
		}

		var v1 configData
		err = ConfigDecPrivate(key, &v1)
		if err != nil {
			t.Fatal(err)
		}

		assert.Equal(t, v1.Name, "ava")
		assert.Equal(t, v1.Age, 1)
	}
}
