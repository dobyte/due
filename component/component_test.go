package component

import "testing"

// customComponent is a component that embeds Base and overrides its name.
type customComponent struct {
	Base
}

// Name returns the component name.
func (c *customComponent) Name() string { return "custom" }

// TestBaseImplementsComponent verifies that Base satisfies the Component interface.
func TestBaseImplementsComponent(t *testing.T) {
	var c Component = &Base{}

	if got := c.Name(); got != "base" {
		t.Errorf("invalid base name, expect: base, actual: %s", got)
	}

	// The remaining lifecycle methods are no-ops and must not panic.
	c.Init()
	c.Start()
	c.Close()
	c.Destroy()
}

// TestCustomComponentOverridesName verifies that embedding Base only provides defaults for the
// methods a component does not override.
func TestCustomComponentOverridesName(t *testing.T) {
	var c Component = &customComponent{}

	if got := c.Name(); got != "custom" {
		t.Errorf("invalid custom name, expect: custom, actual: %s", got)
	}
}
