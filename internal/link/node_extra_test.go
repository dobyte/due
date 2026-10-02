package link

import (
	"bytes"
	"context"
	"testing"
	"time"

	"github.com/dobyte/due/v2/cluster"
	"github.com/dobyte/due/v2/core/buffer"
	"github.com/dobyte/due/v2/errors"
	"github.com/dobyte/due/v2/internal/transporter/node"
	"github.com/dobyte/due/v2/locate"
	"github.com/dobyte/due/v2/registry"
)

// newNodeLinker returns a NodeLinker backed by the base options and a stub locator.
func newNodeLinker() *NodeLinker {
	opts := baseLinkOptions()
	opts.Locator = &locatorStub{}
	return NewNodeLinker(context.Background(), opts)
}

// TestNodeLinkerNew verifies that a new NodeLinker initializes its shards and components.
func TestNodeLinkerNew(t *testing.T) {
	linker := NewNodeLinker(context.Background(), baseLinkOptions())

	if linker.dispatcher == nil {
		t.Errorf("expect a non-nil dispatcher")
	}
	if linker.builder == nil {
		t.Errorf("expect a non-nil builder")
	}

	for i := range linker.shards {
		if linker.shards[i].sources == nil {
			t.Fatalf("shard %d is not initialized", i)
		}
	}

	if linker.shard(0) != linker.shard(sourceShardNum) {
		t.Errorf("expect shard(0) and shard(%d) to be the same shard", sourceShardNum)
	}
	if linker.shard(0) == linker.shard(1) {
		t.Errorf("expect shard(0) and shard(1) to differ")
	}
}

// TestNodeLinkerHasNode verifies node presence lookups against the dispatcher.
func TestNodeLinkerHasNode(t *testing.T) {
	linker := NewNodeLinker(context.Background(), baseLinkOptions())
	linker.dispatcher.ReplaceServices(nodeService("node-1", "127.0.0.1:1"))

	if !linker.HasNode("node-1") {
		t.Errorf("expect node-1 to be present")
	}
	if linker.HasNode("node-2") {
		t.Errorf("expect node-2 to be absent")
	}
}

// TestNodeLinkerLocateNode verifies the locate cache, the locator path and each error branch.
func TestNodeLinkerLocateNode(t *testing.T) {
	t.Run("no locator", func(t *testing.T) {
		linker := NewNodeLinker(context.Background(), baseLinkOptions())

		if _, err := linker.LocateNode(context.Background(), 1, "grp"); !errors.Is(err, errors.ErrNotFoundLocator) {
			t.Errorf("expect ErrNotFoundLocator, got %v", err)
		}
	})

	t.Run("cached source", func(t *testing.T) {
		linker := newNodeLinker()
		linker.doStoreSource(1, "grp", "node-1")

		nid, err := linker.LocateNode(context.Background(), 1, "grp")
		if err != nil {
			t.Fatalf("LocateNode failed: %v", err)
		}
		if nid != "node-1" {
			t.Errorf("nid = %q, want %q", nid, "node-1")
		}
	})

	t.Run("locator hit", func(t *testing.T) {
		opts := baseLinkOptions()
		opts.Locator = &locatorStub{locateNodeFn: func(context.Context, int64, string) (string, error) {
			return "node-9", nil
		}}

		linker := NewNodeLinker(context.Background(), opts)

		nid, err := linker.LocateNode(context.Background(), 2, "grp")
		if err != nil {
			t.Fatalf("LocateNode failed: %v", err)
		}
		if nid != "node-9" {
			t.Errorf("nid = %q, want %q", nid, "node-9")
		}
		if cached, ok := linker.doLoadSource(2, "grp"); !ok || cached != "node-9" {
			t.Errorf("expect the located node to be cached")
		}
	})

	t.Run("empty location", func(t *testing.T) {
		opts := baseLinkOptions()
		opts.Locator = &locatorStub{locateNodeFn: func(context.Context, int64, string) (string, error) {
			return "", nil
		}}

		linker := NewNodeLinker(context.Background(), opts)

		if _, err := linker.LocateNode(context.Background(), 3, "grp"); !errors.Is(err, errors.ErrNotFoundUserLocation) {
			t.Errorf("expect ErrNotFoundUserLocation, got %v", err)
		}
	})

	t.Run("locator error", func(t *testing.T) {
		opts := baseLinkOptions()
		opts.Locator = &locatorStub{locateNodeFn: func(context.Context, int64, string) (string, error) {
			return "", errors.ErrUnknownError
		}}

		linker := NewNodeLinker(context.Background(), opts)

		if _, err := linker.LocateNode(context.Background(), 4, "grp"); !errors.Is(err, errors.ErrUnknownError) {
			t.Errorf("expect ErrUnknownError, got %v", err)
		}
	})
}

// TestNodeLinkerAskNode verifies the locate-comparison result.
func TestNodeLinkerAskNode(t *testing.T) {
	t.Run("no locator", func(t *testing.T) {
		linker := NewNodeLinker(context.Background(), baseLinkOptions())

		if _, _, err := linker.AskNode(context.Background(), 1, "grp", "node-1"); !errors.Is(err, errors.ErrNotFoundLocator) {
			t.Errorf("expect ErrNotFoundLocator, got %v", err)
		}
	})

	tests := []struct {
		name    string
		locate  string
		nid     string
		want    bool
		wantErr error
	}{
		{name: "on node", locate: "node-1", nid: "node-1", want: true},
		{name: "elsewhere", locate: "node-1", nid: "node-2", want: false},
		{name: "empty location", nid: "node-1", wantErr: errors.ErrNotFoundUserLocation},
		{name: "locator error", nid: "node-1", wantErr: errors.ErrUnknownError},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			opts := baseLinkOptions()
			opts.Locator = &locatorStub{locateNodeFn: func(context.Context, int64, string) (string, error) {
				switch tt.wantErr {
				case errors.ErrNotFoundUserLocation:
					return "", nil
				case errors.ErrUnknownError:
					return "", errors.ErrUnknownError
				default:
					return tt.locate, nil
				}
			}}

			linker := NewNodeLinker(context.Background(), opts)

			_, ok, err := linker.AskNode(context.Background(), 1, "grp", tt.nid)
			if tt.wantErr != nil {
				if !errors.Is(err, tt.wantErr) {
					t.Errorf("expect %v, got %v", tt.wantErr, err)
				}
				return
			}
			if err != nil {
				t.Fatalf("AskNode failed: %v", err)
			}
			if ok != tt.want {
				t.Errorf("ok = %v, want %v", ok, tt.want)
			}
		})
	}

	t.Run("cached source", func(t *testing.T) {
		linker := newNodeLinker()
		linker.doStoreSource(1, "grp", "node-1")

		insID, ok, err := linker.AskNode(context.Background(), 1, "grp", "node-1")
		if err != nil {
			t.Fatalf("AskNode failed: %v", err)
		}
		if insID != "node-1" || !ok {
			t.Errorf("AskNode = (%q, %v), want (%q, true)", insID, ok, "node-1")
		}
	})
}

// TestNodeLinkerLocateNodes verifies locating every node of a user.
func TestNodeLinkerLocateNodes(t *testing.T) {
	t.Run("no locator", func(t *testing.T) {
		linker := NewNodeLinker(context.Background(), baseLinkOptions())

		if _, err := linker.LocateNodes(context.Background(), 1); !errors.Is(err, errors.ErrNotFoundLocator) {
			t.Errorf("expect ErrNotFoundLocator, got %v", err)
		}
	})

	t.Run("success", func(t *testing.T) {
		opts := baseLinkOptions()
		opts.Locator = &locatorStub{locateNodesFn: func(context.Context, int64) (map[string]string, error) {
			return map[string]string{"grp": "node-1"}, nil
		}}

		linker := NewNodeLinker(context.Background(), opts)

		nodes, err := linker.LocateNodes(context.Background(), 1)
		if err != nil {
			t.Fatalf("LocateNodes failed: %v", err)
		}
		if nodes["grp"] != "node-1" {
			t.Errorf("nodes = %v, want grp->node-1", nodes)
		}
	})

	t.Run("error", func(t *testing.T) {
		opts := baseLinkOptions()
		opts.Locator = &locatorStub{locateNodesFn: func(context.Context, int64) (map[string]string, error) {
			return nil, errors.ErrUnknownError
		}}

		linker := NewNodeLinker(context.Background(), opts)

		if _, err := linker.LocateNodes(context.Background(), 1); !errors.Is(err, errors.ErrUnknownError) {
			t.Errorf("expect ErrUnknownError, got %v", err)
		}
	})
}

// TestNodeLinkerBindNode verifies binding through the locator and the error branches.
func TestNodeLinkerBindNode(t *testing.T) {
	t.Run("no locator", func(t *testing.T) {
		linker := NewNodeLinker(context.Background(), baseLinkOptions())

		if err := linker.BindNode(context.Background(), 1, "grp", "node-1"); !errors.Is(err, errors.ErrNotFoundLocator) {
			t.Errorf("expect ErrNotFoundLocator, got %v", err)
		}
	})

	t.Run("locator error", func(t *testing.T) {
		opts := baseLinkOptions()
		opts.Locator = &locatorStub{bindNodeFn: func(context.Context, int64, string, string) error {
			return errors.ErrUnknownError
		}}

		linker := NewNodeLinker(context.Background(), opts)

		if err := linker.BindNode(context.Background(), 1, "grp", "node-1"); !errors.Is(err, errors.ErrUnknownError) {
			t.Errorf("expect ErrUnknownError, got %v", err)
		}
	})

	t.Run("success", func(t *testing.T) {
		linker := newNodeLinker()

		if err := linker.BindNode(context.Background(), 1, "grp", "node-1"); err != nil {
			t.Fatalf("BindNode failed: %v", err)
		}
		if nid, ok := linker.doLoadSource(1, "grp"); !ok || nid != "node-1" {
			t.Errorf("expect the binding to be cached")
		}
	})
}

// TestNodeLinkerUnbindNode verifies unbinding through the locator and the error branches.
func TestNodeLinkerUnbindNode(t *testing.T) {
	t.Run("no locator", func(t *testing.T) {
		linker := NewNodeLinker(context.Background(), baseLinkOptions())

		if err := linker.UnbindNode(context.Background(), 1, "grp", "node-1"); !errors.Is(err, errors.ErrNotFoundLocator) {
			t.Errorf("expect ErrNotFoundLocator, got %v", err)
		}
	})

	t.Run("locator error", func(t *testing.T) {
		opts := baseLinkOptions()
		opts.Locator = &locatorStub{unbindNodeFn: func(context.Context, int64, string, string) error {
			return errors.ErrUnknownError
		}}

		linker := NewNodeLinker(context.Background(), opts)

		if err := linker.UnbindNode(context.Background(), 1, "grp", "node-1"); !errors.Is(err, errors.ErrUnknownError) {
			t.Errorf("expect ErrUnknownError, got %v", err)
		}
	})

	t.Run("success", func(t *testing.T) {
		linker := newNodeLinker()
		linker.doStoreSource(1, "grp", "node-1")

		if err := linker.UnbindNode(context.Background(), 1, "grp", "node-1"); err != nil {
			t.Fatalf("UnbindNode failed: %v", err)
		}
		if _, ok := linker.doLoadSource(1, "grp"); ok {
			t.Errorf("expect the binding to be removed")
		}
	})

	t.Run("mismatched nid is ignored", func(t *testing.T) {
		linker := newNodeLinker()
		linker.doStoreSource(1, "grp", "node-1")

		if err := linker.UnbindNode(context.Background(), 1, "grp", "node-2"); err != nil {
			t.Fatalf("UnbindNode failed: %v", err)
		}
		if nid, ok := linker.doLoadSource(1, "grp"); !ok || nid != "node-1" {
			t.Errorf("expect the binding to survive a mismatched unbind")
		}
	})
}

// TestNodeLinkerFetchNodeList verifies node listing and state filtering.
func TestNodeLinkerFetchNodeList(t *testing.T) {
	services := []*registry.ServiceInstance{
		{ID: "node-1", State: cluster.Work.String()},
		{ID: "node-2", State: cluster.Busy.String()},
	}

	tests := []struct {
		name   string
		states []cluster.State
		want   int
	}{
		{name: "no filter", states: nil, want: 2},
		{name: "work only", states: []cluster.State{cluster.Work}, want: 1},
		{name: "work and busy", states: []cluster.State{cluster.Work, cluster.Busy}, want: 2},
		{name: "no match", states: []cluster.State{cluster.Hang}, want: 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			opts := baseLinkOptions()
			opts.Registry = &registryStub{servicesFn: func(context.Context, string) ([]*registry.ServiceInstance, error) {
				return services, nil
			}}

			linker := NewNodeLinker(context.Background(), opts)

			list, err := linker.FetchNodeList(context.Background(), tt.states...)
			if err != nil {
				t.Fatalf("FetchNodeList failed: %v", err)
			}
			if len(list) != tt.want {
				t.Errorf("len(list) = %d, want %d", len(list), tt.want)
			}
		})
	}

	t.Run("registry error", func(t *testing.T) {
		opts := baseLinkOptions()
		opts.Registry = &registryStub{servicesFn: func(context.Context, string) ([]*registry.ServiceInstance, error) {
			return nil, errors.ErrRegistryClosed
		}}

		linker := NewNodeLinker(context.Background(), opts)

		if _, err := linker.FetchNodeList(context.Background()); !errors.Is(err, errors.ErrRegistryClosed) {
			t.Errorf("expect ErrRegistryClosed, got %v", err)
		}
	})
}

// TestNodeLinkerState verifies SetState against a live node server plus the error branches of
// GetState and SetState.
//
// As with the gate linker, a successful GetState is unreachable through the transporter because the
// get-state request carries an empty private section that drpc drops as a close signal.
func TestNodeLinkerState(t *testing.T) {
	provider := &nodeProviderStub{}
	_, addr := startNodeServer(t, provider)

	linker := NewNodeLinker(context.Background(), baseLinkOptions())
	linker.dispatcher.ReplaceServices(nodeService("node-1", addr))

	if err := linker.SetState(context.Background(), "node-1", cluster.Busy); err != nil {
		t.Fatalf("SetState failed: %v", err)
	}
	if got := cluster.State(provider.setStateVal.Load()); got != cluster.Busy {
		t.Errorf("provider state = %v, want %v", got, cluster.Busy)
	}

	if _, err := linker.GetState(context.Background(), ""); !errors.Is(err, errors.ErrInvalidNID) {
		t.Errorf("expect ErrInvalidNID, got %v", err)
	}
	if _, err := linker.GetState(context.Background(), "missing"); !errors.Is(err, errors.ErrNotFoundEndpoint) {
		t.Errorf("expect ErrNotFoundEndpoint, got %v", err)
	}
	if err := linker.SetState(context.Background(), "missing", cluster.Work); !errors.Is(err, errors.ErrNotFoundEndpoint) {
		t.Errorf("expect ErrNotFoundEndpoint, got %v", err)
	}
}

// TestNodeLinkerDeliver verifies the deliver paths: direct addressing by NID, routed delivery and
// every buffer kind, together with the error branches.
func TestNodeLinkerDeliver(t *testing.T) {
	t.Run("direct by nid", func(t *testing.T) {
		provider := &nodeProviderStub{deliver: make(chan nodeLinkDeliverRecord, 4)}
		_, addr := startNodeServer(t, provider)

		linker := NewNodeLinker(context.Background(), baseLinkOptions())
		linker.dispatcher.ReplaceServices(nodeService("node-1", addr))

		tests := []struct {
			name   string
			buffer any
			want   string
			packed bool
		}{
			{name: "bytes", buffer: []byte("bytes-payload"), want: "bytes-payload"},
			{name: "message", buffer: &Message{Route: 1, Data: []byte("message-payload")}, want: "message-payload", packed: true},
			{name: "buffer", buffer: buffer.NewBytes([]byte("buffer-payload")), want: "buffer-payload"},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				args := &DeliverArgs{NID: "node-1", CID: 10, UID: 20, Buffer: tt.buffer}
				if err := linker.Deliver(context.Background(), args); err != nil {
					t.Fatalf("Deliver failed: %v", err)
				}

				select {
				case rec := <-provider.deliver:
					if rec.gid != "test-gate" || rec.cid != 10 || rec.uid != 20 {
						t.Errorf("unexpected deliver record: %+v", rec)
					}
					// A *Message is packed into the wire envelope, so only the payload is checked.
					if tt.packed {
						if !bytes.Contains(rec.data, []byte(tt.want)) {
							t.Errorf("data = %v, want it to contain %q", rec.data, tt.want)
						}
					} else if string(rec.data) != tt.want {
						t.Errorf("data = %v, want %q", rec.data, tt.want)
					}
				case <-time.After(2 * time.Second):
					t.Fatal("provider did not receive the message in time")
				}
			})
		}

		t.Run("unknown nid", func(t *testing.T) {
			args := &DeliverArgs{NID: "missing", CID: 10, UID: 20, Buffer: []byte("x")}
			if err := linker.Deliver(context.Background(), args); !errors.Is(err, errors.ErrNotFoundEndpoint) {
				t.Errorf("expect ErrNotFoundEndpoint, got %v", err)
			}
		})
	})

	t.Run("invalid buffer", func(t *testing.T) {
		linker := NewNodeLinker(context.Background(), baseLinkOptions())

		args := &DeliverArgs{NID: "node-1", CID: 10, UID: 20, Buffer: 123}
		if err := linker.Deliver(context.Background(), args); !errors.Is(err, errors.ErrInvalidMessage) {
			t.Errorf("expect ErrInvalidMessage, got %v", err)
		}
	})

	t.Run("routed stateless", func(t *testing.T) {
		provider := &nodeProviderStub{deliver: make(chan nodeLinkDeliverRecord, 1)}
		_, addr := startNodeServer(t, provider)

		linker := NewNodeLinker(context.Background(), baseLinkOptions())
		linker.dispatcher.ReplaceServices(nodeService("node-1", addr, registry.Route{ID: 1}))

		args := &DeliverArgs{Route: 1, CID: 10, UID: 20, Buffer: []byte("routed")}
		if err := linker.Deliver(context.Background(), args); err != nil {
			t.Fatalf("Deliver failed: %v", err)
		}

		select {
		case rec := <-provider.deliver:
			if string(rec.data) != "routed" {
				t.Errorf("data = %q, want %q", rec.data, "routed")
			}
		case <-time.After(2 * time.Second):
			t.Fatal("provider did not receive the message in time")
		}
	})

	t.Run("routed stateful", func(t *testing.T) {
		provider := &nodeProviderStub{deliver: make(chan nodeLinkDeliverRecord, 1)}
		_, addr := startNodeServer(t, provider)

		opts := baseLinkOptions()
		opts.Locator = &locatorStub{locateNodeFn: func(context.Context, int64, string) (string, error) {
			return "node-1", nil
		}}

		linker := NewNodeLinker(context.Background(), opts)
		linker.dispatcher.ReplaceServices(nodeService("node-1", addr, registry.Route{ID: 2, Stateful: true}))

		args := &DeliverArgs{Route: 2, CID: 10, UID: 20, Buffer: []byte("stateful")}
		if err := linker.Deliver(context.Background(), args); err != nil {
			t.Fatalf("Deliver failed: %v", err)
		}

		select {
		case rec := <-provider.deliver:
			if string(rec.data) != "stateful" {
				t.Errorf("data = %q, want %q", rec.data, "stateful")
			}
		case <-time.After(2 * time.Second):
			t.Fatal("provider did not receive the message in time")
		}
	})

	t.Run("missing user location is swallowed", func(t *testing.T) {
		opts := baseLinkOptions()
		opts.Locator = &locatorStub{locateNodeFn: func(context.Context, int64, string) (string, error) {
			return "", errors.ErrNotFoundUserLocation
		}}

		linker := NewNodeLinker(context.Background(), opts)
		linker.dispatcher.ReplaceServices(nodeService("node-1", "127.0.0.1:1", registry.Route{ID: 2, Stateful: true}))

		args := &DeliverArgs{Route: 2, CID: 10, UID: 20, Buffer: []byte("x")}
		if err := linker.Deliver(context.Background(), args); err != nil {
			t.Errorf("expect the missing location error to be swallowed, got %v", err)
		}
	})

	t.Run("route not found", func(t *testing.T) {
		linker := NewNodeLinker(context.Background(), baseLinkOptions())

		args := &DeliverArgs{Route: 99, CID: 10, UID: 20, Buffer: []byte("x")}
		if err := linker.Deliver(context.Background(), args); !errors.Is(err, errors.ErrNotFoundRoute) {
			t.Errorf("expect ErrNotFoundRoute, got %v", err)
		}
	})

	t.Run("pack error", func(t *testing.T) {
		opts := baseLinkOptions()
		opts.Codec = &codecStub{marshalFn: func(any) ([]byte, error) { return nil, errors.ErrInvalidFormat }}

		linker := NewNodeLinker(context.Background(), opts)

		args := &DeliverArgs{NID: "node-1", CID: 10, UID: 20, Buffer: &Message{Route: 1, Data: struct{}{}}}
		if err := linker.Deliver(context.Background(), args); !errors.Is(err, errors.ErrInvalidFormat) {
			t.Errorf("expect ErrInvalidFormat, got %v", err)
		}
	})
}

// TestNodeLinkerDoBuildClient verifies building a node client for valid and invalid instance IDs.
func TestNodeLinkerDoBuildClient(t *testing.T) {
	provider := &nodeProviderStub{}
	_, addr := startNodeServer(t, provider)

	linker := NewNodeLinker(context.Background(), baseLinkOptions())
	linker.dispatcher.ReplaceServices(nodeService("node-1", addr))

	if _, err := linker.doBuildClient(""); !errors.Is(err, errors.ErrInvalidNID) {
		t.Errorf("expect ErrInvalidNID, got %v", err)
	}
	if _, err := linker.doBuildClient("missing"); !errors.Is(err, errors.ErrNotFoundEndpoint) {
		t.Errorf("expect ErrNotFoundEndpoint, got %v", err)
	}
	client, err := linker.doBuildClient("node-1")
	if err != nil {
		t.Fatalf("doBuildClient failed: %v", err)
	}
	if client == nil {
		t.Errorf("expect a non-nil client")
	}
}

// TestNodeLinkerDoRPC verifies the guard clauses and retry logic of the node RPC helper.
func TestNodeLinkerDoRPC(t *testing.T) {
	t.Run("route not found", func(t *testing.T) {
		linker := NewNodeLinker(context.Background(), baseLinkOptions())

		if _, err := linker.doRPC(context.Background(), 99, 1, func(context.Context, *node.Client) (bool, any, error) {
			return false, nil, nil
		}); !errors.Is(err, errors.ErrNotFoundRoute) {
			t.Errorf("expect ErrNotFoundRoute, got %v", err)
		}
	})

	t.Run("stateful zero uid", func(t *testing.T) {
		linker := NewNodeLinker(context.Background(), baseLinkOptions())
		linker.dispatcher.ReplaceServices(nodeService("node-1", "127.0.0.1:1", registry.Route{ID: 2, Stateful: true}))

		if _, err := linker.doRPC(context.Background(), 2, 0, func(context.Context, *node.Client) (bool, any, error) {
			return false, nil, nil
		}); !errors.Is(err, errors.ErrIllegalRequest) {
			t.Errorf("expect ErrIllegalRequest, got %v", err)
		}
	})

	t.Run("authorized zero uid", func(t *testing.T) {
		linker := NewNodeLinker(context.Background(), baseLinkOptions())
		linker.dispatcher.ReplaceServices(nodeService("node-1", "127.0.0.1:1", registry.Route{ID: 3, Authorized: true}))

		if _, err := linker.doRPC(context.Background(), 3, 0, func(context.Context, *node.Client) (bool, any, error) {
			return false, nil, nil
		}); !errors.Is(err, errors.ErrIllegalRequest) {
			t.Errorf("expect ErrIllegalRequest, got %v", err)
		}
	})

	t.Run("internal route from gate", func(t *testing.T) {
		linker := NewNodeLinker(context.Background(), baseLinkOptions())
		linker.dispatcher.ReplaceServices(nodeService("node-1", "127.0.0.1:1", registry.Route{ID: 4, Internal: true}))

		if _, err := linker.doRPC(context.Background(), 4, 1, func(context.Context, *node.Client) (bool, any, error) {
			return false, nil, nil
		}); !errors.Is(err, errors.ErrIllegalRequest) {
			t.Errorf("expect ErrIllegalRequest, got %v", err)
		}
	})

	t.Run("stateful locate error", func(t *testing.T) {
		opts := baseLinkOptions()
		opts.Locator = &locatorStub{locateNodeFn: func(context.Context, int64, string) (string, error) {
			return "", errors.ErrNotFoundUserLocation
		}}

		linker := NewNodeLinker(context.Background(), opts)
		linker.dispatcher.ReplaceServices(nodeService("node-1", "127.0.0.1:1", registry.Route{ID: 2, Stateful: true}))

		if _, err := linker.doRPC(context.Background(), 2, 1, func(context.Context, *node.Client) (bool, any, error) {
			return false, nil, nil
		}); !errors.Is(err, errors.ErrNotFoundUserLocation) {
			t.Errorf("expect ErrNotFoundUserLocation, got %v", err)
		}
	})

	t.Run("stateful endpoint not found", func(t *testing.T) {
		opts := baseLinkOptions()
		opts.Locator = &locatorStub{locateNodeFn: func(context.Context, int64, string) (string, error) {
			return "missing", nil
		}}

		linker := NewNodeLinker(context.Background(), opts)
		linker.dispatcher.ReplaceServices(nodeService("node-1", "127.0.0.1:1", registry.Route{ID: 2, Stateful: true}))

		if _, err := linker.doRPC(context.Background(), 2, 1, func(context.Context, *node.Client) (bool, any, error) {
			return false, nil, nil
		}); !errors.Is(err, errors.ErrNotFoundEndpoint) {
			t.Errorf("expect ErrNotFoundEndpoint, got %v", err)
		}
	})

	t.Run("retry with unchanged node", func(t *testing.T) {
		provider := &nodeProviderStub{}
		_, addr := startNodeServer(t, provider)

		var calls int
		opts := baseLinkOptions()
		opts.Locator = &locatorStub{locateNodeFn: func(context.Context, int64, string) (string, error) {
			calls++
			return "node-1", nil
		}}

		linker := NewNodeLinker(context.Background(), opts)
		linker.dispatcher.ReplaceServices(nodeService("node-1", addr, registry.Route{ID: 2, Stateful: true}))

		// The first invocation asks for a retry; the second locate resolves to the same node, so
		// doRPC returns without invoking fn again.
		var invoked int
		_, err := linker.doRPC(context.Background(), 2, 1, func(context.Context, *node.Client) (bool, any, error) {
			invoked++
			return true, "reply", nil
		})
		if err != nil {
			t.Errorf("expect nil error, got %v", err)
		}
		if invoked != 1 {
			t.Errorf("invoked = %d, want 1", invoked)
		}
		if calls != 2 {
			t.Errorf("locate calls = %d, want 2", calls)
		}
	})

	t.Run("build error", func(t *testing.T) {
		linker := NewNodeLinker(context.Background(), baseLinkOptions())
		linker.dispatcher.ReplaceServices(nodeService("bad", "127.0.0.1:99999", registry.Route{ID: 1}))

		if _, err := linker.doRPC(context.Background(), 1, 1, func(context.Context, *node.Client) (bool, any, error) {
			return false, nil, nil
		}); err == nil {
			t.Errorf("expect a build error for an invalid port")
		}
	})
}

// TestNodeLinkerPackMessage verifies message and buffer packing, including encryption and errors.
func TestNodeLinkerPackMessage(t *testing.T) {
	t.Run("buffer variants", func(t *testing.T) {
		opts := baseLinkOptions()
		opts.Encryptor = encryptorStub{}
		linker := NewNodeLinker(context.Background(), opts)

		tests := []struct {
			name    string
			message any
			encrypt bool
			want    string
			wantNil bool
		}{
			{name: "nil", message: nil, encrypt: true, wantNil: true},
			{name: "bytes", message: []byte("raw"), encrypt: true, want: "raw"},
			{name: "struct", message: map[string]int{"a": 1}, encrypt: false, want: `{"a":1}`},
			{name: "struct encrypted", message: map[string]int{"a": 1}, encrypt: true, want: `enc:{"a":1}`},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				data, err := linker.doPackBuffer(tt.message, tt.encrypt)
				if err != nil {
					t.Fatalf("doPackBuffer failed: %v", err)
				}
				if tt.wantNil {
					if data != nil {
						t.Errorf("data = %v, want nil", data)
					}
					return
				}
				if string(data) != tt.want {
					t.Errorf("data = %q, want %q", data, tt.want)
				}
			})
		}
	})

	t.Run("marshal error", func(t *testing.T) {
		opts := baseLinkOptions()
		opts.Codec = &codecStub{marshalFn: func(any) ([]byte, error) { return nil, errors.ErrInvalidFormat }}
		linker := NewNodeLinker(context.Background(), opts)

		if _, err := linker.doPackBuffer(struct{}{}, false); !errors.Is(err, errors.ErrInvalidFormat) {
			t.Errorf("expect ErrInvalidFormat, got %v", err)
		}
	})

	t.Run("pack message", func(t *testing.T) {
		linker := NewNodeLinker(context.Background(), baseLinkOptions())

		buf, err := linker.PackMessage(&Message{Seq: 1, Route: 2, Data: []byte("payload")}, true)
		if err != nil {
			t.Fatalf("PackMessage failed: %v", err)
		}
		if buf == nil || buf.Len() == 0 {
			t.Errorf("expect a non-empty packed buffer")
		}
		buf.Release()
	})

	t.Run("pack message error", func(t *testing.T) {
		opts := baseLinkOptions()
		opts.Codec = &codecStub{marshalFn: func(any) ([]byte, error) { return nil, errors.ErrInvalidFormat }}
		linker := NewNodeLinker(context.Background(), opts)

		if _, err := linker.PackMessage(&Message{Route: 1, Data: struct{}{}}, false); !errors.Is(err, errors.ErrInvalidFormat) {
			t.Errorf("expect ErrInvalidFormat, got %v", err)
		}
	})
}

// TestNodeLinkerSourceCache verifies the store, delete and load behaviour of the sharded cache,
// including the wait and done handlers.
func TestNodeLinkerSourceCache(t *testing.T) {
	t.Run("store scenarios", func(t *testing.T) {
		tests := []struct {
			name     string
			preload  func(*NodeLinker)
			store    func(*NodeLinker)
			wantWait int
			wantDone int
			check    func(*testing.T, *NodeLinker)
		}{
			{
				name:     "new uid yields wait when it is the local instance",
				store:    func(l *NodeLinker) { l.doStoreSource(1, "grp", "me") },
				wantWait: 1,
				check: func(t *testing.T, l *NodeLinker) {
					if nid, ok := l.doLoadSource(1, "grp"); !ok || nid != "me" {
						t.Errorf("expect grp->me")
					}
				},
			},
			{
				name:     "new name on existing uid yields wait",
				preload:  func(l *NodeLinker) { l.doStoreSource(1, "other", "x") },
				store:    func(l *NodeLinker) { l.doStoreSource(1, "grp", "me") },
				wantWait: 1,
				check:    func(t *testing.T, l *NodeLinker) {},
			},
			{
				name:     "same nid is a no-op",
				preload:  func(l *NodeLinker) { l.doStoreSource(1, "grp", "me") },
				store:    func(l *NodeLinker) { l.doStoreSource(1, "grp", "me") },
				wantWait: 1,
			},
			{
				name:     "replacing the local instance yields done",
				preload:  func(l *NodeLinker) { l.doStoreSource(1, "grp", "me") },
				store:    func(l *NodeLinker) { l.doStoreSource(1, "grp", "other") },
				wantWait: 1,
				wantDone: 1,
				check: func(t *testing.T, l *NodeLinker) {
					if nid, ok := l.doLoadSource(1, "grp"); !ok || nid != "other" {
						t.Errorf("expect grp->other")
					}
				},
			},
			{
				name:     "replacing a remote instance with the local one yields wait",
				preload:  func(l *NodeLinker) { l.doStoreSource(1, "grp", "other") },
				store:    func(l *NodeLinker) { l.doStoreSource(1, "grp", "me") },
				wantWait: 1,
			},
			{
				name:     "replacing a remote instance with another remote yields nothing",
				preload:  func(l *NodeLinker) { l.doStoreSource(1, "grp", "other") },
				store:    func(l *NodeLinker) { l.doStoreSource(1, "grp", "third") },
				wantWait: 0,
			},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				var wait, done int

				opts := baseLinkOptions()
				opts.ID = "me"
				opts.WaitHandler = func() bool { wait++; return true }
				opts.DoneHandler = func() bool { done++; return true }

				linker := NewNodeLinker(context.Background(), opts)

				if tt.preload != nil {
					tt.preload(linker)
				}
				tt.store(linker)

				if wait != tt.wantWait {
					t.Errorf("wait handler calls = %d, want %d", wait, tt.wantWait)
				}
				if done != tt.wantDone {
					t.Errorf("done handler calls = %d, want %d", done, tt.wantDone)
				}
				if tt.check != nil {
					tt.check(t, linker)
				}
			})
		}
	})

	t.Run("delete scenarios", func(t *testing.T) {
		tests := []struct {
			name     string
			preload  func(*NodeLinker)
			del      func(*NodeLinker)
			wantDone int
			check    func(*testing.T, *NodeLinker)
		}{
			{
				name: "unknown uid",
				del:  func(l *NodeLinker) { l.doDeleteSource(1, "grp", "me") },
			},
			{
				name: "unknown name",
				preload: func(l *NodeLinker) {
					l.doStoreSource(1, "grp", "me")
				},
				del:      func(l *NodeLinker) { l.doDeleteSource(1, "other", "me") },
				wantDone: 0,
				check: func(t *testing.T, l *NodeLinker) {
					if _, ok := l.doLoadSource(1, "grp"); !ok {
						t.Errorf("expect the binding to survive")
					}
				},
			},
			{
				name:     "mismatched nid",
				preload:  func(l *NodeLinker) { l.doStoreSource(1, "grp", "me") },
				del:      func(l *NodeLinker) { l.doDeleteSource(1, "grp", "other") },
				wantDone: 0,
				check: func(t *testing.T, l *NodeLinker) {
					if _, ok := l.doLoadSource(1, "grp"); !ok {
						t.Errorf("expect the binding to survive a mismatched nid")
					}
				},
			},
			{
				name:     "last binding of the local instance yields done",
				preload:  func(l *NodeLinker) { l.doStoreSource(1, "grp", "me") },
				del:      func(l *NodeLinker) { l.doDeleteSource(1, "grp", "me") },
				wantDone: 1,
				check: func(t *testing.T, l *NodeLinker) {
					if _, ok := l.doLoadSource(1, "grp"); ok {
						t.Errorf("expect the binding to be removed")
					}
				},
			},
			{
				name: "one of several bindings yields done only for the local instance",
				preload: func(l *NodeLinker) {
					l.doStoreSource(1, "grp", "me")
					l.doStoreSource(1, "other", "x")
				},
				del:      func(l *NodeLinker) { l.doDeleteSource(1, "grp", "me") },
				wantDone: 1,
				check: func(t *testing.T, l *NodeLinker) {
					if _, ok := l.doLoadSource(1, "grp"); ok {
						t.Errorf("expect grp to be removed")
					}
					if nid, ok := l.doLoadSource(1, "other"); !ok || nid != "x" {
						t.Errorf("expect other->x to survive")
					}
				},
			},
			{
				name: "one of several bindings yields no done for a remote instance",
				preload: func(l *NodeLinker) {
					l.doStoreSource(1, "grp", "other")
					l.doStoreSource(1, "second", "x")
				},
				del:      func(l *NodeLinker) { l.doDeleteSource(1, "grp", "other") },
				wantDone: 0,
				check: func(t *testing.T, l *NodeLinker) {
					if _, ok := l.doLoadSource(1, "grp"); ok {
						t.Errorf("expect grp to be removed")
					}
				},
			},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				var done int

				opts := baseLinkOptions()
				opts.ID = "me"
				opts.DoneHandler = func() bool { done++; return true }

				linker := NewNodeLinker(context.Background(), opts)

				if tt.preload != nil {
					tt.preload(linker)
				}
				tt.del(linker)

				if done != tt.wantDone {
					t.Errorf("done handler calls = %d, want %d", done, tt.wantDone)
				}
				if tt.check != nil {
					tt.check(t, linker)
				}
			})
		}
	})

	t.Run("load scenarios", func(t *testing.T) {
		linker := newNodeLinker()

		if _, ok := linker.doLoadSource(1, "grp"); ok {
			t.Errorf("expect an unknown source to be absent")
		}

		linker.doStoreSource(1, "grp", "node-1")

		if nid, ok := linker.doLoadSource(1, "grp"); !ok || nid != "node-1" {
			t.Errorf("doLoadSource = (%q, %v), want (node-1, true)", nid, ok)
		}
		if _, ok := linker.doLoadSource(1, "other"); ok {
			t.Errorf("expect an unknown name to be absent")
		}
	})
}

// TestNodeLinkerWatchUserLocate verifies the locate event watcher cache maintenance.
func TestNodeLinkerWatchUserLocate(t *testing.T) {
	t.Run("no locator", func(t *testing.T) {
		linker := NewNodeLinker(context.Background(), baseLinkOptions())
		linker.WatchUserLocate()

		if _, ok := linker.doLoadSource(1, "grp"); ok {
			t.Errorf("expect no sources without a locator")
		}
	})

	t.Run("events", func(t *testing.T) {
		watcher := &locateWatcherStub{steps: make(chan locateWatchStep, 6)}
		watcher.steps <- locateWatchStep{err: errors.ErrUnknownError}
		watcher.steps <- locateWatchStep{events: []*locate.Event{{UID: 1, Type: locate.BindNode, InsName: "grp", InsID: "node-1"}}}
		watcher.steps <- locateWatchStep{events: []*locate.Event{{UID: 3, Type: locate.BindNode, InsName: "grp", InsID: "node-3"}}}
		watcher.steps <- locateWatchStep{events: []*locate.Event{{UID: 1, Type: locate.UnbindNode, InsName: "grp", InsID: "node-1"}}}
		watcher.steps <- locateWatchStep{events: []*locate.Event{{UID: 4, Type: locate.EventType(0), InsName: "grp", InsID: "ignored"}}}
		watcher.steps <- locateWatchStep{err: errors.ErrWatcherStopped}

		opts := baseLinkOptions()
		opts.Locator = &locatorStub{watchFn: func(context.Context, ...string) (locate.Watcher, error) {
			return watcher, nil
		}}

		linker := NewNodeLinker(context.Background(), opts)
		linker.WatchUserLocate()

		if !waitFor(t, 2*time.Second, func() bool {
			nid, ok := linker.doLoadSource(3, "grp")
			return ok && nid == "node-3"
		}) {
			t.Fatalf("expect uid 3 to be cached on node-3")
		}

		if !waitFor(t, 2*time.Second, func() bool {
			_, ok := linker.doLoadSource(1, "grp")
			return !ok
		}) {
			t.Errorf("expect uid 1 to be removed by the unbind event")
		}

		if _, ok := linker.doLoadSource(4, "grp"); ok {
			t.Errorf("expect the unknown event type to be ignored")
		}
	})
}

// TestNodeLinkerWatchClusterInstance verifies the cluster instance watcher dispatcher refresh.
func TestNodeLinkerWatchClusterInstance(t *testing.T) {
	watcher := &registryWatcherStub{steps: make(chan registryWatchStep, 4)}
	watcher.steps <- registryWatchStep{err: errors.ErrUnknownError}
	watcher.steps <- registryWatchStep{services: []*registry.ServiceInstance{nodeService("node-1", "127.0.0.1:1")}}
	watcher.steps <- registryWatchStep{services: []*registry.ServiceInstance{nodeService("node-2", "127.0.0.1:2")}}
	watcher.steps <- registryWatchStep{err: context.Canceled}

	opts := baseLinkOptions()
	opts.Registry = &registryStub{watchFn: func(context.Context, string) (registry.Watcher, error) {
		return watcher, nil
	}}

	linker := NewNodeLinker(context.Background(), opts)
	linker.WatchClusterInstance()

	if !waitFor(t, 2*time.Second, func() bool { return linker.HasNode("node-2") }) {
		t.Fatalf("expect node-2 to be registered")
	}
	if linker.HasNode("node-1") {
		t.Errorf("expect node-1 to be replaced by the newest service list")
	}
}
