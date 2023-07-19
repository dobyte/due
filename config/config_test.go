package config_test

import (
	"github.com/dobyte/due/v2/config"
	"testing"
)

func TestConfig(t *testing.T) {
	//v := config.Get("c.redis.addrs.1A", "192.168.0.1:3308").String()
	//t.Log(v)

	//t.Log(config.Get("config.packet").Map())

	//t.Log(config.Has("config.packet"))
	//
	//t.Log(config.Has("config.notFound"))

	//ticker1 := time.NewTicker(2 * time.Second)
	//ticker2 := time.After(time.Minute)
	//
	//for {
	//	select {
	//	case <-ticker1.C:
	//		t.Log(config.Get("config.packet").Map())
	//	case <-ticker2:
	//		config.Close()
	//		return
	//	}
	//}

	////config.Set("c.redis.addrs.1.name", 1)
	//config.Set("c.redis.addrs.5", "192.168.0.1:3308")
	//v = config.Get("c.redis.addrs.5").String()
	//t.Log(v)
	//
	//select {}
}

func BenchmarkGet(b *testing.B) {
	for i := 0; i < b.N; i++ {
		config.Get("config").Value()
	}

	//b.Logf("%+v", config.Get("config").Map())
}
