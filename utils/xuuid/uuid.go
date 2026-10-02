// Package xuuid provides UUID generation utilities.
package xuuid

import "uuid"

// UUID returns a new version 7 UUID string.
//
// A version 7 UUID is generated from a millisecond timestamp and random bits, so it is roughly
// monotonic in time. Compared with the fully random version 4, it is better suited as a database
// primary key or a distributed ID because it reduces index fragmentation.
func UUID() string {
	return uuid.NewV7().String()
}
