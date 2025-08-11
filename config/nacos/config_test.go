package nacos_test

import (
	"context"
	"testing"
	"time"

	"github.com/dobyte/due/config/nacos/v2"
	"github.com/dobyte/due/v2/config"
)

func init() {
	source := nacos.NewSource()
	config.SetConfigurator(config.NewConfigurator(config.WithSources(source)))
}

func TestWatch(t *testing.T) {
	ticker1 := time.NewTicker(2 * time.Second)
	ticker2 := time.After(20 * time.Minute)

	for {
		select {
		case <-ticker1.C:
			t.Log(config.Get("config.timezone").String())
			t.Log(config.Get("config.pid").String())
		case <-ticker2:
			config.Close()
			return
		}
	}
}

func TestLoad(t *testing.T) {
	ctx := context.Background()
	file := "config.json"
	c, err := config.Load(ctx, nacos.Name, file)
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
	file := "configs.json"
	content1 := map[string]any{
		"timezone": "Local",
	}

	content2 := map[string]any{
		"timezone": "UTC",
		"pid":      "./run/gate.pid",
	}

	err := config.Store(ctx, nacos.Name, file, content1, true)
	if err != nil {
		t.Fatal(err)
	}

	time.Sleep(5 * time.Second)

	err = config.Store(ctx, nacos.Name, file, content2)
	if err != nil {
		t.Fatal(err)
	}
}

func BenchmarkGet(b *testing.B) {
	for i := 0; i < b.N; i++ {
		config.Get("config").Value()
	}
}
