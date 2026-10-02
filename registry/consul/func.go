package consul

import (
	"fmt"

	"github.com/dobyte/due/v2/registry"
)

// makeInsID builds the instance ID.
//
// It concatenates the instance name and ID so that instances with the same ID but different service
// names do not overwrite each other.
func makeInsID(ins *registry.ServiceInstance) string {
	return fmt.Sprintf("%s-%s", ins.Name, ins.ID)
}
