package config_test

import (
	"github.com/dobyte/due/config"
	"testing"
)

func TestConfig(t *testing.T) {
	v := config.Get("c.redis.addrs.1A", "192.168.0.1:3308").String()
	t.Log(v)

	//config.Set("c.redis.addrs.1.name", 1)
	config.Set("c.redis.addrs.5", "192.168.0.1:3308")
	v = config.Get("c.redis.addrs.5").String()
	t.Log(v)
}
