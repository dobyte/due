package polaris

import "testing"

func TestOptionsNormalize(t *testing.T) {
	o := &options{}
	o.normalize()

	if o.logDir != defaultLogDir {
		t.Fatalf("normalize() logDir = %q, want %q", o.logDir, defaultLogDir)
	}

	if o.cacheDir != defaultCacheDir {
		t.Fatalf("normalize() cacheDir = %q, want %q", o.cacheDir, defaultCacheDir)
	}

	o = &options{logDir: "/tmp/log", cacheDir: "/tmp/cache"}
	o.normalize()

	if o.logDir != "/tmp/log" || o.cacheDir != "/tmp/cache" {
		t.Fatalf("normalize() = %+v, want the configured directories", o)
	}
}
