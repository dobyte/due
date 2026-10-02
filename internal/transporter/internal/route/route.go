package route

const (
	Handshake   uint8 = iota + 1 // Handshake
	Bind                         // Bind a user
	Unbind                       // Unbind a user
	GetIP                        // Get the IP address
	Stat                         // Count online sessions
	IsOnline                     // Check whether a user is online
	Disconnect                   // Disconnect
	Push                         // Push a single message
	Multicast                    // Push a multicast message
	Broadcast                    // Push a broadcast message
	Publish                      // Publish a channel event
	Subscribe                    // Subscribe to a channel
	Unsubscribe                  // Unsubscribe from a channel
	Trigger                      // Trigger an event
	Deliver                      // Deliver a message
	GetState                     // Get the state
	SetState                     // Set the state
)
