package config_test

import (
	"github.com/dobyte/due/config"
	"testing"
)

func TestConfig(t *testing.T) {
	v := config.Get("c.redis.addrs.1A", "192.168.0.1:3308").String()
	t.Log(v)

	//config.Set("c.redis.addrs.1.name", 1)
	config.Set("c.redis.addrs.3.name", 2)

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
