package xuuid

import (
	"github.com/google/uuid"
)

func UUID() string {
	id, _ := uuid.NewUUID()
	return id.String()
}
