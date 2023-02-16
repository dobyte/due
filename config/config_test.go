package config_test

import (
	"github.com/dobyte/due/config"
	"testing"
)

func TestConfig(t *testing.T) {
	//v := config.Get("c.redis.addrs.1A", "192.168.0.1:3308").String()
	//t.Log(v)

	t.Log(config.Get("config.packet").Map())

	t.Log(config.Has("config.packet"))

	t.Log(config.Has("config.notFound"))

	//ticker1 := time.NewTicker(2 * time.Second)
	//ticker2 := time.NewTicker(10 * time.Second)
	//
	//for {
	//	select {
	//	case <-ticker1.C:
	//		t.Log(config.Get("wechat.1.db", 0))
	//	case <-ticker2.C:
	//		config.Close()
	//		ticker1.Close()
	//		ticker2.Close()
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
		config.Get("config").Map()
	}

	//b.Logf("%+v", config.Get("config").Map())
}
