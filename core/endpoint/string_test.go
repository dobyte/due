package endpoint_test

import (
	"testing"

	"github.com/dobyte/due/v2/core/endpoint"
)

func TestEndpointStringPreservesDiscoveryMetadata(t *testing.T) {
	const target = "tcp://[::1]:3554?due.bridge=a%2Fb&is_secure=true"
	e, err := endpoint.ParseEndpoint(target)
	if err != nil {
		t.Fatal(err)
	}
	if e.String() != target || e.Address() != "[::1]:3554" || !e.IsSecure() {
		t.Fatalf("endpoint lost discovery metadata: %s", e.String())
	}
	const created = "tcp://[::1]:3554?is_secure=true"
	if got := endpoint.NewEndpoint("tcp", "[::1]:3554", true).String(); got != created {
		t.Fatalf("created endpoint %q, want %q", got, created)
	}
}

func BenchmarkEndpointString(b *testing.B) {
	e, err := endpoint.ParseEndpoint("tcp://127.0.0.1:3554?due.bridge=process.incarnation&is_secure=false")
	if err != nil {
		b.Fatal(err)
	}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if e.String() == "" {
			b.Fatal("empty endpoint")
		}
	}
}
