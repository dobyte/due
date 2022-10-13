package config_test

import (
	"github.com/dobyte/due/config"
	"testing"
	"time"
)

func TestConfig(t *testing.T) {
	//v := config.Get("c.redis.addrs.1A", "192.168.0.1:3308").String()
	//t.Log(v)

	ticker1 := time.NewTicker(2 * time.Second)
	ticker2 := time.NewTicker(10 * time.Second)

	for {
		select {
		case <-ticker1.C:
			t.Log(config.Get("b.0.age", 0))
		case <-ticker2.C:
			config.Close()
			ticker1.Stop()
			ticker2.Stop()
			return
		}
	}

	////config.Set("c.redis.addrs.1.name", 1)
	//config.Set("c.redis.addrs.5", "192.168.0.1:3308")
	//v = config.Get("c.redis.addrs.5").String()
	//t.Log(v)
	//
	//select {}
}
