package polaris

import "testing"

func TestDefaultCacheAndLogDirs(t *testing.T) {
	o := defaultOptions()

	if o.cacheDir != defaultCacheDir {
		t.Fatalf("invalid cache dir, expect: %s, actual: %s", defaultCacheDir, o.cacheDir)
	}

	if o.logDir != defaultLogDir {
		t.Fatalf("invalid log dir, expect: %s, actual: %s", defaultLogDir, o.logDir)
	}
}

func TestWithCacheAndLogDirs(t *testing.T) {
	o := defaultOptions()

	WithCacheDir("/tmp/polaris/cache")(o)
	WithLogDir("/tmp/polaris/log")(o)

	if o.cacheDir != "/tmp/polaris/cache" {
		t.Fatalf("invalid cache dir, actual: %s", o.cacheDir)
	}

	if o.logDir != "/tmp/polaris/log" {
		t.Fatalf("invalid log dir, actual: %s", o.logDir)
	}
}
