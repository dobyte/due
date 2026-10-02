package node

import "testing"

// TestBaseProcessor verifies the empty lifecycle callbacks of the base processor.
func TestBaseProcessor(t *testing.T) {
	var processor BaseProcessor

	processor.Init()
	processor.Start()
	processor.Destroy()
}
