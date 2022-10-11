package config_test

import (
	"github.com/dobyte/due/config"
	"testing"
)

func TestConfig(t *testing.T) {
	v := config.Get("c.c.d").Int()
	t.Log(v)

	//reader := config.NewReader(config.WithSources(
	//	config.NewSource("./config"),
	//))

	//config.Load("config").SetSource()

	//source := config.NewSource("./config")
	//
	//configurations, err := source.Load()
	//if err != nil {
	//	t.Fatal(err)
	//}
	//
	//t.Logf("%+v", configurations)
}
