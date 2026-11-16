//go:build integration
// +build integration

// These tests require a running Consul server (defaulting to 127.0.0.1:8500) and are excluded
// from the default test run. Run them with:
//
//	go test -tags integration ./...
package consul_test

import (
	"context"
	"testing"
	"time"

	"github.com/dobyte/due/config/consul/v2"
	"github.com/dobyte/due/v2/config"
	"github.com/dobyte/due/v2/errors"
)

func init() {
	source := consul.NewSource(consul.WithMode(config.ReadWrite))
	config.SetConfigurator(config.NewConfigurator(config.WithSources(source)))
}

func TestWatch(t *testing.T) {
	ticker1 := time.NewTicker(2 * time.Second)
	ticker2 := time.After(time.Minute)

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
	c, err := config.Load(ctx, consul.Name, file)
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
	content1 := map[string]any{
		"timezone": "Local",
	}

	content2 := map[string]any{
		"timezone": "UTC",
		"pid":      "./run/gate.pid",
	}

	err := config.Store(ctx, consul.Name, file, content1, true)
	if err != nil {
		t.Fatal(err)
	}

	time.Sleep(5 * time.Second)

	err = config.Store(ctx, consul.Name, file, content2)
	if err != nil {
		t.Fatal(err)
	}
}

// TestFiles verifies that an explicit load or store outside the configured file list is rejected.
func TestFiles(t *testing.T) {
	source := consul.NewSource(consul.WithMode(config.ReadWrite), consul.WithFiles("config.json"))
	defer source.Close()

	ctx := context.Background()

	if _, err := source.Load(ctx, "gate.json"); !errors.Is(err, errors.ErrFileNotConfigured) {
		t.Fatalf("expect ErrFileNotConfigured, actual: %v", err)
	}

	if err := source.Store(ctx, "gate.json", []byte(`{}`)); !errors.Is(err, errors.ErrFileNotConfigured) {
		t.Fatalf("expect ErrFileNotConfigured, actual: %v", err)
	}
}

func BenchmarkGet(b *testing.B) {
	for b.Loop() {
		config.Get("config").Value()
	}
}
