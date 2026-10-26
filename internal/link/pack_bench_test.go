package link

import (
	"context"
	"strings"
	"testing"
)

type benchPayload struct {
	Message string `json:"message"`
	SentAt  int64  `json:"sentAt"`
}

// BenchmarkGateLinkerPackMessage measures the JSON pack path for a typical business payload.
func BenchmarkGateLinkerPackMessage(b *testing.B) {
	linker := NewGateLinker(context.Background(), baseLinkOptions())

	tests := []struct {
		name string
		size int
	}{
		{name: "1KB", size: 1024},
		{name: "4KB", size: 4 * 1024},
	}

	for _, tt := range tests {
		b.Run(tt.name, func(b *testing.B) {
			message := &Message{
				Route: 1,
				Seq:   1,
				Data: &benchPayload{
					Message: strings.Repeat("x", tt.size),
					SentAt:  1234567890,
				},
			}

			b.ReportAllocs()
			b.ResetTimer()

			for b.Loop() {
				buf, err := linker.PackMessage(message, true)
				if err != nil {
					b.Fatalf("PackMessage failed: %v", err)
				}

				buf.Release()
			}
		})
	}
}
