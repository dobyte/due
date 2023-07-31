package config_test

import (
	"context"
	"github.com/dobyte/due/config/etcd/v2"
	"github.com/dobyte/due/v2/config"
	"github.com/dobyte/due/v2/config/configurator"
	"testing"
	"time"
)

func init() {
	config.SetConfigurator(configurator.NewConfigurator(configurator.WithSources(etcd.NewSource())))
}

func TestWatch(t *testing.T) {
	ticker1 := time.NewTicker(2 * time.Second)
	ticker2 := time.After(time.Minute)

	for {
		select {
		case <-ticker1.C:
			t.Log(config.Get("config.timezone").String())
		case <-ticker2:
			config.Close()
			return
		}
	}
}

func TestLoad(t *testing.T) {
	ctx := context.Background()
	file := "config.json"
	c, err := config.Load(ctx, etcd.Name, file)
	if err != nil {
		t.Fatal(err)
	}

	t.Log(c[0].Name)
	t.Log(c[0].Path)
	t.Log(c[0].Format)
	t.Log(c[0].Content)
}

func TestStore(t *testing.T) {
	ctx := context.Background()
	file := "config.json"
	content := map[string]interface{}{
		"timezone": "Local",
	}

	err := config.Store(ctx, etcd.Name, file, content)
	if err != nil {
		t.Fatal(err)
	}
}

func BenchmarkGet(b *testing.B) {
	for i := 0; i < b.N; i++ {
		config.Get("config").Value()
	}
}
