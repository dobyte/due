package polaris_test

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/dobyte/due/config/polaris/v2"
	"github.com/dobyte/due/v2/config"
)

func init() {
	source := polaris.NewSource(
		polaris.WithMode(config.ReadWrite),
	)
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
	t.Log(config.Get("config.timezone").String())
	t.Log(config.Get("config.pid").String())
}

func TestStore(t *testing.T) {
	ctx := context.Background()
	file := "config.json"
	content1 := map[string]any{
		"timezone": "Local",
	}

	content2 := map[string]any{
		"timezone": "UTC",
		"pid":      "./run/gate.pid",
	}

	err := config.Store(ctx, polaris.Name, file, content1, true)
	if err != nil {
		t.Fatal(err)
	}

	time.Sleep(5 * time.Second)

	err = config.Store(ctx, polaris.Name, file, content2)
	if err != nil {
		t.Fatal(err)
	}
}

func TestFullChain(t *testing.T) {
	ctx := context.Background()
	// Use a unique file name to avoid interference from historical test data.
	file := fmt.Sprintf("config-%d.json", time.Now().UnixNano())
	configName := strings.TrimSuffix(file, ".json")

	// 1. Store the configuration for the first time (create and publish).
	err := config.Store(ctx, polaris.Name, file, map[string]any{
		"timezone": "Local",
		"pid":      "./run/gate.pid",
	}, true)
	if err != nil {
		t.Fatal(err)
	}
	t.Log("store ok")

	// 2. Load it directly to verify, which also subscribes to the config file.
	deadline := time.Now().Add(30 * time.Second)
	for {
		cs, err := config.Load(ctx, polaris.Name, file)
		if err == nil && len(cs) == 1 && string(cs[0].Content) != "" {
			t.Logf("load ok, name=%s format=%s content=%s", cs[0].Name, cs[0].Format, string(cs[0].Content))
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("load config timeout")
		}
		time.Sleep(500 * time.Millisecond)
	}

	// 3. Store it again (update and publish) to trigger a server push.
	err = config.Store(ctx, polaris.Name, file, map[string]any{
		"timezone": "UTC",
		"pid":      "./run/gate.pid",
	}, true)
	if err != nil {
		t.Fatal(err)
	}

	// 4. Wait for the change to propagate through the watch chain and verify the hot update.
	deadline = time.Now().Add(30 * time.Second)
	for {
		if timezone := config.Get(configName + ".timezone").String(); timezone == "UTC" {
			t.Logf("watch hot update ok, timezone=%s", timezone)
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("watch hot update timeout")
		}
		time.Sleep(500 * time.Millisecond)
	}
}

func BenchmarkGet(b *testing.B) {
	for b.Loop() {
		config.Get("config").Value()
	}
}
