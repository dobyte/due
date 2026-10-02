package link

import (
	"context"
	"testing"
	"time"

	"github.com/dobyte/due/v2/cluster"
	"github.com/dobyte/due/v2/core/buffer"
	"github.com/dobyte/due/v2/errors"
	"github.com/dobyte/due/v2/locate"
	"github.com/dobyte/due/v2/registry"
	"github.com/dobyte/due/v2/session"
)

// TestGateLinkerNew verifies that a new GateLinker has its dispatcher and builder initialized.
func TestGateLinkerNew(t *testing.T) {
	linker := NewGateLinker(context.Background(), baseLinkOptions())

	if linker.dispatcher == nil {
		t.Errorf("expect a non-nil dispatcher")
	}
	if linker.builder == nil {
		t.Errorf("expect a non-nil builder")
	}
}

// TestGateLinkerHasGate verifies gate presence lookups against the dispatcher.
func TestGateLinkerHasGate(t *testing.T) {
	linker := NewGateLinker(context.Background(), baseLinkOptions())
	linker.dispatcher.ReplaceServices(gateService("gate-1", "127.0.0.1:1"))

	tests := []struct {
		name string
		gid  string
		want bool
	}{
		{name: "registered gate", gid: "gate-1", want: true},
		{name: "unknown gate", gid: "gate-2", want: false},
		{name: "empty gate", gid: "", want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := linker.HasGate(tt.gid); got != tt.want {
				t.Errorf("HasGate(%q) = %v, want %v", tt.gid, got, tt.want)
			}
		})
	}
}

// TestGateLinkerFetchGateList verifies gate listing and state filtering.
func TestGateLinkerFetchGateList(t *testing.T) {
	services := []*registry.ServiceInstance{
		{ID: "gate-1", State: cluster.Work.String()},
		{ID: "gate-2", State: cluster.Busy.String()},
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

			linker := NewGateLinker(context.Background(), opts)

			list, err := linker.FetchGateList(context.Background(), tt.states...)
			if err != nil {
				t.Fatalf("FetchGateList failed: %v", err)
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

		linker := NewGateLinker(context.Background(), opts)

		if _, err := linker.FetchGateList(context.Background()); !errors.Is(err, errors.ErrRegistryClosed) {
			t.Errorf("expect ErrRegistryClosed, got %v", err)
		}
	})
}

// TestGateLinkerLocateGate verifies the locate cache, the locator path and each error branch.
func TestGateLinkerLocateGate(t *testing.T) {
	t.Run("no locator", func(t *testing.T) {
		linker := NewGateLinker(context.Background(), baseLinkOptions())

		if _, err := linker.LocateGate(context.Background(), 1); !errors.Is(err, errors.ErrNotFoundLocator) {
			t.Errorf("expect ErrNotFoundLocator, got %v", err)
		}
	})

	t.Run("cached source", func(t *testing.T) {
		linker := newGateLinker()
		linker.sources.Store(int64(1), "gate-1")

		gid, err := linker.LocateGate(context.Background(), 1)
		if err != nil {
			t.Fatalf("LocateGate failed: %v", err)
		}
		if gid != "gate-1" {
			t.Errorf("gid = %q, want %q", gid, "gate-1")
		}
	})

	t.Run("locator hit", func(t *testing.T) {
		opts := baseLinkOptions()
		opts.Locator = &locatorStub{locateGateFn: func(context.Context, int64) (string, error) {
			return "gate-9", nil
		}}

		linker := NewGateLinker(context.Background(), opts)

		gid, err := linker.LocateGate(context.Background(), 2)
		if err != nil {
			t.Fatalf("LocateGate failed: %v", err)
		}
		if gid != "gate-9" {
			t.Errorf("gid = %q, want %q", gid, "gate-9")
		}
		if _, ok := linker.sources.Load(int64(2)); !ok {
			t.Errorf("expect the located gate to be cached")
		}
	})

	t.Run("empty location", func(t *testing.T) {
		opts := baseLinkOptions()
		opts.Locator = &locatorStub{locateGateFn: func(context.Context, int64) (string, error) {
			return "", nil
		}}

		linker := NewGateLinker(context.Background(), opts)

		if _, err := linker.LocateGate(context.Background(), 3); !errors.Is(err, errors.ErrNotFoundUserLocation) {
			t.Errorf("expect ErrNotFoundUserLocation, got %v", err)
		}
	})

	t.Run("locator error", func(t *testing.T) {
		opts := baseLinkOptions()
		opts.Locator = &locatorStub{locateGateFn: func(context.Context, int64) (string, error) {
			return "", errors.ErrUnknownError
		}}

		linker := NewGateLinker(context.Background(), opts)

		if _, err := linker.LocateGate(context.Background(), 4); !errors.Is(err, errors.ErrUnknownError) {
			t.Errorf("expect ErrUnknownError, got %v", err)
		}
	})
}

// TestGateLinkerAskGate verifies the gate comparison result.
func TestGateLinkerAskGate(t *testing.T) {
	tests := []struct {
		name    string
		locate  string
		gid     string
		want    bool
		wantErr error
	}{
		{name: "located on gate", locate: "gate-1", gid: "gate-1", want: true},
		{name: "located elsewhere", locate: "gate-1", gid: "gate-2", want: false},
		{name: "locate error", locate: "", gid: "gate-1", wantErr: errors.ErrUnknownError},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			opts := baseLinkOptions()
			opts.Locator = &locatorStub{locateGateFn: func(context.Context, int64) (string, error) {
				if tt.locate == "" {
					return "", errors.ErrUnknownError
				}
				return tt.locate, nil
			}}

			linker := NewGateLinker(context.Background(), opts)

			_, ok, err := linker.AskGate(context.Background(), tt.gid, 1)
			if tt.wantErr != nil {
				if !errors.Is(err, tt.wantErr) {
					t.Errorf("expect %v, got %v", tt.wantErr, err)
				}
				return
			}
			if err != nil {
				t.Fatalf("AskGate failed: %v", err)
			}
			if ok != tt.want {
				t.Errorf("ok = %v, want %v", ok, tt.want)
			}
		})
	}
}

// TestGateLinkerBindGate verifies binding through a live gate server plus the error branches.
func TestGateLinkerBindGate(t *testing.T) {
	provider := &gateProviderStub{}
	_, addr := startGateServer(t, provider)

	linker := newGateLinker()
	linker.dispatcher.ReplaceServices(gateService("gate-1", addr))

	if err := linker.BindGate(context.Background(), "gate-1", 100, 200); err != nil {
		t.Fatalf("BindGate failed: %v", err)
	}

	if got := provider.bindCount.Load(); got != 1 {
		t.Errorf("bind count = %d, want 1", got)
	}
	if gid, err := linker.LocateGate(context.Background(), 200); err != nil || gid != "gate-1" {
		t.Errorf("expected uid 200 to be cached on gate-1, got gid=%q err=%v", gid, err)
	}

	t.Run("invalid gid", func(t *testing.T) {
		if err := linker.BindGate(context.Background(), "", 100, 200); !errors.Is(err, errors.ErrInvalidGID) {
			t.Errorf("expect ErrInvalidGID, got %v", err)
		}
	})

	t.Run("unknown gid", func(t *testing.T) {
		if err := linker.BindGate(context.Background(), "missing", 100, 200); !errors.Is(err, errors.ErrNotFoundEndpoint) {
			t.Errorf("expect ErrNotFoundEndpoint, got %v", err)
		}
	})

	t.Run("provider error", func(t *testing.T) {
		failing := &gateProviderStub{bindFn: func(context.Context, int64, int64) error {
			return errors.ErrNotFoundSession
		}}
		_, addr := startGateServer(t, failing)

		linker := newGateLinker()
		linker.dispatcher.ReplaceServices(gateService("gate-1", addr))

		if err := linker.BindGate(context.Background(), "gate-1", 100, 300); !errors.Is(err, errors.ErrNotFoundSession) {
			t.Errorf("expect ErrNotFoundSession, got %v", err)
		}
	})
}

// TestGateLinkerUnbindGate verifies the RPC retry and failure handling of UnbindGate.
func TestGateLinkerUnbindGate(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		provider := &gateProviderStub{}
		_, addr := startGateServer(t, provider)

		linker := newGateLinker()
		linker.dispatcher.ReplaceServices(gateService("gate-1", addr))
		linker.sources.Store(int64(200), "gate-1")

		if err := linker.UnbindGate(context.Background(), 200); err != nil {
			t.Fatalf("UnbindGate failed: %v", err)
		}
		if _, ok := linker.sources.Load(int64(200)); ok {
			t.Errorf("expect the source to be removed")
		}
	})

	t.Run("retry with unchanged gate", func(t *testing.T) {
		provider := &gateProviderStub{unbindFn: func(context.Context, int64) error {
			return errors.ErrNotFoundSession
		}}
		_, addr := startGateServer(t, provider)

		opts := baseLinkOptions()
		opts.Locator = &locatorStub{locateGateFn: func(context.Context, int64) (string, error) {
			return "gate-1", nil
		}}

		linker := NewGateLinker(context.Background(), opts)
		linker.dispatcher.ReplaceServices(gateService("gate-1", addr))

		// When the second locate resolves to the same gate, doRPC stops without issuing another
		// call. LocateGate succeeds and resets the returned error to nil, so UnbindGate reports
		// success even though the gate reported the session missing.
		if err := linker.UnbindGate(context.Background(), 200); err != nil {
			t.Errorf("expect nil, got %v", err)
		}
		if got := provider.unbindCount.Load(); got != 1 {
			t.Errorf("unbind count = %d, want 1", got)
		}
	})

	t.Run("retry with changed gate", func(t *testing.T) {
		provider1 := &gateProviderStub{unbindFn: func(context.Context, int64) error {
			return errors.ErrNotFoundSession
		}}
		_, addr1 := startGateServer(t, provider1)

		provider2 := &gateProviderStub{}
		_, addr2 := startGateServer(t, provider2)

		var calls int
		opts := baseLinkOptions()
		opts.Locator = &locatorStub{locateGateFn: func(context.Context, int64) (string, error) {
			calls++
			if calls == 1 {
				return "gate-1", nil
			}
			return "gate-2", nil
		}}

		linker := NewGateLinker(context.Background(), opts)
		linker.dispatcher.ReplaceServices(gateService("gate-1", addr1), gateService("gate-2", addr2))

		if err := linker.UnbindGate(context.Background(), 200); err != nil {
			t.Fatalf("UnbindGate failed: %v", err)
		}
		if got := provider2.unbindCount.Load(); got != 1 {
			t.Errorf("second gate unbind count = %d, want 1", got)
		}
	})

	t.Run("locate error", func(t *testing.T) {
		opts := baseLinkOptions()
		opts.Locator = &locatorStub{locateGateFn: func(context.Context, int64) (string, error) {
			return "", errors.ErrNotFoundUserLocation
		}}

		linker := NewGateLinker(context.Background(), opts)

		if err := linker.UnbindGate(context.Background(), 200); !errors.Is(err, errors.ErrNotFoundUserLocation) {
			t.Errorf("expect ErrNotFoundUserLocation, got %v", err)
		}
	})

	t.Run("build error", func(t *testing.T) {
		opts := baseLinkOptions()
		opts.Locator = &locatorStub{locateGateFn: func(context.Context, int64) (string, error) {
			return "missing-gate", nil
		}}

		linker := NewGateLinker(context.Background(), opts)

		if err := linker.UnbindGate(context.Background(), 200); !errors.Is(err, errors.ErrNotFoundEndpoint) {
			t.Errorf("expect ErrNotFoundEndpoint, got %v", err)
		}
	})
}

// TestGateLinkerState verifies SetState against a live gate server plus the error branches of
// GetState and SetState.
//
// A successful GetState is not reachable through the transporter: the get-state request carries an
// empty private section, which drpc interprets as a close signal and drops. Only SetState (which
// carries the state byte) can round-trip, so GetState is covered through its error branch.
func TestGateLinkerState(t *testing.T) {
	provider := &gateProviderStub{}
	_, addr := startGateServer(t, provider)

	linker := NewGateLinker(context.Background(), baseLinkOptions())
	linker.dispatcher.ReplaceServices(gateService("gate-1", addr))

	if err := linker.SetState(context.Background(), "gate-1", cluster.Busy); err != nil {
		t.Fatalf("SetState failed: %v", err)
	}
	provider.mu.Lock()
	gotState := provider.lastState
	provider.mu.Unlock()
	if gotState != cluster.Busy {
		t.Errorf("provider state = %v, want %v", gotState, cluster.Busy)
	}

	if _, err := linker.GetState(context.Background(), "missing"); !errors.Is(err, errors.ErrNotFoundEndpoint) {
		t.Errorf("expect ErrNotFoundEndpoint, got %v", err)
	}
	if err := linker.SetState(context.Background(), "missing", cluster.Work); !errors.Is(err, errors.ErrNotFoundEndpoint) {
		t.Errorf("expect ErrNotFoundEndpoint, got %v", err)
	}
}

// TestGateLinkerGetIP verifies direct, indirect and invalid-kind IP lookups.
func TestGateLinkerGetIP(t *testing.T) {
	provider := &gateProviderStub{}
	_, addr := startGateServer(t, provider)

	linker := newGateLinker()
	linker.dispatcher.ReplaceServices(gateService("gate-1", addr))
	linker.sources.Store(int64(200), "gate-1")

	tests := []struct {
		name    string
		args    *GetIPArgs
		want    string
		wantErr error
	}{
		{name: "conn direct", args: &GetIPArgs{GID: "gate-1", Kind: session.Conn, Target: 100}, want: "127.0.0.1"},
		{name: "user direct", args: &GetIPArgs{GID: "gate-1", Kind: session.User, Target: 200}, want: "127.0.0.1"},
		{name: "user indirect", args: &GetIPArgs{Kind: session.User, Target: 200}, want: "127.0.0.1"},
		{name: "invalid kind", args: &GetIPArgs{Kind: session.Kind(99), Target: 100}, wantErr: errors.ErrInvalidSessionKind},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ip, err := linker.GetIP(context.Background(), tt.args)
			if tt.wantErr != nil {
				if !errors.Is(err, tt.wantErr) {
					t.Errorf("expect %v, got %v", tt.wantErr, err)
				}
				return
			}
			if err != nil {
				t.Fatalf("GetIP failed: %v", err)
			}
			if ip != tt.want {
				t.Errorf("ip = %q, want %q", ip, tt.want)
			}
		})
	}

	t.Run("direct build error", func(t *testing.T) {
		if _, err := linker.GetIP(context.Background(), &GetIPArgs{GID: "missing", Kind: session.Conn, Target: 1}); !errors.Is(err, errors.ErrNotFoundEndpoint) {
			t.Errorf("expect ErrNotFoundEndpoint, got %v", err)
		}
	})
}

// TestGateLinkerStat verifies session counting across gate endpoints.
func TestGateLinkerStat(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		provider := &gateProviderStub{statFn: func(context.Context, session.Kind) (int64, error) {
			return 7, nil
		}}
		_, addr := startGateServer(t, provider)

		linker := NewGateLinker(context.Background(), baseLinkOptions())
		linker.dispatcher.ReplaceServices(gateService("gate-1", addr))

		total, err := linker.Stat(context.Background(), session.Conn)
		if err != nil {
			t.Fatalf("Stat failed: %v", err)
		}
		if total != 7 {
			t.Errorf("total = %d, want 7", total)
		}
	})

	t.Run("error", func(t *testing.T) {
		provider := &gateProviderStub{statFn: func(context.Context, session.Kind) (int64, error) {
			return 0, errors.ErrNotFoundSession
		}}
		_, addr := startGateServer(t, provider)

		linker := NewGateLinker(context.Background(), baseLinkOptions())
		linker.dispatcher.ReplaceServices(gateService("gate-1", addr))

		total, err := linker.Stat(context.Background(), session.Conn)
		if !errors.Is(err, errors.ErrNotFoundSession) {
			t.Errorf("expect ErrNotFoundSession, got %v", err)
		}
		if total != 0 {
			t.Errorf("total = %d, want 0", total)
		}
	})

	t.Run("no endpoints", func(t *testing.T) {
		linker := NewGateLinker(context.Background(), baseLinkOptions())

		total, err := linker.Stat(context.Background(), session.Conn)
		if err != nil || total != 0 {
			t.Errorf("Stat = (%d, %v), want (0, nil)", total, err)
		}
	})

	t.Run("build error", func(t *testing.T) {
		linker := NewGateLinker(context.Background(), baseLinkOptions())
		linker.dispatcher.ReplaceServices(gateService("bad", "127.0.0.1:99999"))

		total, err := linker.Stat(context.Background(), session.Conn)
		if err == nil {
			t.Errorf("expect a build error for an invalid port")
		}
		if total != 0 {
			t.Errorf("total = %d, want 0", total)
		}
	})
}

// TestGateLinkerIsOnline verifies direct, indirect and invalid-kind online checks.
func TestGateLinkerIsOnline(t *testing.T) {
	provider := &gateProviderStub{isOnlineFn: func(context.Context, session.Kind, int64) (bool, error) {
		return true, nil
	}}
	_, addr := startGateServer(t, provider)

	linker := newGateLinker()
	linker.dispatcher.ReplaceServices(gateService("gate-1", addr))
	linker.sources.Store(int64(200), "gate-1")

	tests := []struct {
		name    string
		args    *IsOnlineArgs
		want    bool
		wantErr error
	}{
		{name: "conn direct", args: &IsOnlineArgs{GID: "gate-1", Kind: session.Conn, Target: 100}, want: true},
		{name: "user direct", args: &IsOnlineArgs{GID: "gate-1", Kind: session.User, Target: 200}, want: true},
		{name: "user indirect", args: &IsOnlineArgs{Kind: session.User, Target: 200}, want: true},
		{name: "invalid kind", args: &IsOnlineArgs{Kind: session.Kind(99)}, wantErr: errors.ErrInvalidSessionKind},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ok, err := linker.IsOnline(context.Background(), tt.args)
			if tt.wantErr != nil {
				if !errors.Is(err, tt.wantErr) {
					t.Errorf("expect %v, got %v", tt.wantErr, err)
				}
				return
			}
			if err != nil {
				t.Fatalf("IsOnline failed: %v", err)
			}
			if ok != tt.want {
				t.Errorf("ok = %v, want %v", ok, tt.want)
			}
		})
	}
}

// TestGateLinkerDisconnect verifies direct, indirect and invalid-kind disconnects.
func TestGateLinkerDisconnect(t *testing.T) {
	provider := &gateProviderStub{}
	_, addr := startGateServer(t, provider)

	linker := newGateLinker()
	linker.dispatcher.ReplaceServices(gateService("gate-1", addr))
	linker.sources.Store(int64(200), "gate-1")

	tests := []struct {
		name    string
		args    *DisconnectArgs
		wantErr error
	}{
		{name: "conn direct", args: &DisconnectArgs{GID: "gate-1", Kind: session.Conn, Target: 100}},
		{name: "user direct", args: &DisconnectArgs{GID: "gate-1", Kind: session.User, Target: 200, Force: true}},
		{name: "user indirect", args: &DisconnectArgs{Kind: session.User, Target: 200}},
		{name: "invalid kind", args: &DisconnectArgs{Kind: session.Kind(99)}, wantErr: errors.ErrInvalidSessionKind},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := linker.Disconnect(context.Background(), tt.args)
			if tt.wantErr != nil {
				if !errors.Is(err, tt.wantErr) {
					t.Errorf("expect %v, got %v", tt.wantErr, err)
				}
				return
			}
			if err != nil {
				t.Errorf("Disconnect failed: %v", err)
			}
		})
	}

	if got := provider.disconnectCount.Load(); got != 3 {
		t.Errorf("disconnect count = %d, want 3", got)
	}
}

// TestGateLinkerSubscribe verifies subscribe for direct, indirect, empty and invalid cases.
func TestGateLinkerSubscribe(t *testing.T) {
	provider := &gateProviderStub{}
	_, addr := startGateServer(t, provider)

	linker := newGateLinker()
	linker.dispatcher.ReplaceServices(gateService("gate-1", addr))
	linker.sources.Store(int64(200), "gate-1")

	tests := []struct {
		name    string
		args    *SubscribeArgs
		wantErr error
	}{
		{name: "conn direct", args: &SubscribeArgs{GID: "gate-1", Kind: session.Conn, Targets: []int64{100}, Channel: "room"}},
		{name: "user direct", args: &SubscribeArgs{GID: "gate-1", Kind: session.User, Targets: []int64{200}, Channel: "room"}},
		{name: "user indirect", args: &SubscribeArgs{Kind: session.User, Targets: []int64{200}, Channel: "room"}},
		{name: "empty targets", args: &SubscribeArgs{GID: "gate-1", Kind: session.Conn, Channel: "room"}, wantErr: errors.ErrReceiveTargetEmpty},
		{name: "invalid kind", args: &SubscribeArgs{Kind: session.Kind(99)}, wantErr: errors.ErrInvalidSessionKind},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := linker.Subscribe(context.Background(), tt.args)
			if tt.wantErr != nil {
				if !errors.Is(err, tt.wantErr) {
					t.Errorf("expect %v, got %v", tt.wantErr, err)
				}
				return
			}
			if err != nil {
				t.Errorf("Subscribe failed: %v", err)
			}
		})
	}

	t.Run("direct build error", func(t *testing.T) {
		err := linker.Subscribe(context.Background(), &SubscribeArgs{GID: "missing", Kind: session.Conn, Targets: []int64{1}})
		if !errors.Is(err, errors.ErrNotFoundEndpoint) {
			t.Errorf("expect ErrNotFoundEndpoint, got %v", err)
		}
	})
}

// TestGateLinkerUnsubscribe verifies unsubscribe for direct, indirect, empty and invalid cases.
func TestGateLinkerUnsubscribe(t *testing.T) {
	provider := &gateProviderStub{}
	_, addr := startGateServer(t, provider)

	linker := newGateLinker()
	linker.dispatcher.ReplaceServices(gateService("gate-1", addr))
	linker.sources.Store(int64(200), "gate-1")

	tests := []struct {
		name    string
		args    *UnsubscribeArgs
		wantErr error
	}{
		{name: "conn direct", args: &UnsubscribeArgs{GID: "gate-1", Kind: session.Conn, Targets: []int64{100}, Channel: "room"}},
		{name: "user direct", args: &UnsubscribeArgs{GID: "gate-1", Kind: session.User, Targets: []int64{200}, Channel: "room"}},
		{name: "user indirect", args: &UnsubscribeArgs{Kind: session.User, Targets: []int64{200}, Channel: "room"}},
		{name: "empty targets", args: &UnsubscribeArgs{GID: "gate-1", Kind: session.Conn, Channel: "room"}, wantErr: errors.ErrReceiveTargetEmpty},
		{name: "invalid kind", args: &UnsubscribeArgs{Kind: session.Kind(99)}, wantErr: errors.ErrInvalidSessionKind},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := linker.Unsubscribe(context.Background(), tt.args)
			if tt.wantErr != nil {
				if !errors.Is(err, tt.wantErr) {
					t.Errorf("expect %v, got %v", tt.wantErr, err)
				}
				return
			}
			if err != nil {
				t.Errorf("Unsubscribe failed: %v", err)
			}
		})
	}
}

// TestGateLinkerMulticast verifies direct, indirect, ack and error multicast paths.
func TestGateLinkerMulticast(t *testing.T) {
	provider := &gateProviderStub{}
	_, addr := startGateServer(t, provider)

	linker := newGateLinker()
	linker.dispatcher.ReplaceServices(gateService("gate-1", addr))
	for _, uid := range []int64{200, 201} {
		linker.sources.Store(uid, "gate-1")
	}

	msg := &Message{Route: 1, Data: []byte("hello")}

	// The multi-target indirect path is covered by TestGateLinkerMulticastIndirectFanOut against
	// live gates, so only the single-target and direct paths are exercised here.
	tests := []struct {
		name      string
		args      *MulticastArgs
		wantTotal int64
		wantErr   error
	}{
		{name: "invalid kind", args: &MulticastArgs{Kind: session.Kind(99), Targets: []int64{1}, Message: msg}, wantErr: errors.ErrInvalidSessionKind},
		{name: "conn empty targets", args: &MulticastArgs{GID: "gate-1", Kind: session.Conn, Message: msg}, wantErr: errors.ErrReceiveTargetEmpty},
		{name: "conn single no-ack", args: &MulticastArgs{GID: "gate-1", Kind: session.Conn, Targets: []int64{100}, Message: msg}},
		{name: "conn single ack", args: &MulticastArgs{GID: "gate-1", Kind: session.Conn, Targets: []int64{100}, Message: msg, Ack: true}, wantTotal: 1},
		{name: "conn multi ack", args: &MulticastArgs{GID: "gate-1", Kind: session.Conn, Targets: []int64{100, 101}, Message: msg, Ack: true}, wantTotal: 2},
		{name: "user direct multi ack", args: &MulticastArgs{GID: "gate-1", Kind: session.User, Targets: []int64{200, 201}, Message: msg, Ack: true}, wantTotal: 2},
		{name: "user indirect single no-ack", args: &MulticastArgs{Kind: session.User, Targets: []int64{200}, Message: msg}},
		{name: "user indirect single ack", args: &MulticastArgs{Kind: session.User, Targets: []int64{200}, Message: msg, Ack: true}, wantTotal: 1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			total, err := linker.Multicast(context.Background(), tt.args)
			if tt.wantErr != nil {
				if !errors.Is(err, tt.wantErr) {
					t.Errorf("expect %v, got %v", tt.wantErr, err)
				}
				return
			}
			if err != nil {
				t.Fatalf("Multicast failed: %v", err)
			}
			if total != tt.wantTotal {
				t.Errorf("total = %d, want %d", total, tt.wantTotal)
			}
		})
	}

	t.Run("direct build error", func(t *testing.T) {
		_, err := linker.Multicast(context.Background(), &MulticastArgs{GID: "missing", Kind: session.Conn, Targets: []int64{1}, Message: msg})
		if !errors.Is(err, errors.ErrNotFoundEndpoint) {
			t.Errorf("expect ErrNotFoundEndpoint, got %v", err)
		}
	})
}

// TestGateLinkerPush verifies a single-target push.
func TestGateLinkerPush(t *testing.T) {
	provider := &gateProviderStub{}
	_, addr := startGateServer(t, provider)

	linker := newGateLinker()
	linker.dispatcher.ReplaceServices(gateService("gate-1", addr))
	linker.sources.Store(int64(200), "gate-1")

	t.Run("direct", func(t *testing.T) {
		err := linker.Push(context.Background(), &PushArgs{GID: "gate-1", Kind: session.Conn, Target: 100, Message: &Message{Route: 1, Data: []byte("x")}})
		if err != nil {
			t.Errorf("Push failed: %v", err)
		}
	})

	t.Run("indirect", func(t *testing.T) {
		err := linker.Push(context.Background(), &PushArgs{Kind: session.User, Target: 200, Message: &Message{Route: 1, Data: []byte("x")}})
		if err != nil {
			t.Errorf("Push failed: %v", err)
		}
	})
}

// TestGateLinkerBroadcast verifies broadcast over zero, one and many gate endpoints.
func TestGateLinkerBroadcast(t *testing.T) {
	msg := &Message{Route: 1, Data: []byte("hello")}

	t.Run("no endpoints", func(t *testing.T) {
		linker := NewGateLinker(context.Background(), baseLinkOptions())

		total, err := linker.Broadcast(context.Background(), &BroadcastArgs{Kind: session.Conn, Message: msg})
		if err != nil || total != 0 {
			t.Errorf("Broadcast = (%d, %v), want (0, nil)", total, err)
		}
	})

	t.Run("one gate", func(t *testing.T) {
		provider := &gateProviderStub{broadcastFn: func(context.Context, session.Kind, bool, buffer.Buffer) (int64, error) {
			return 3, nil
		}}
		_, addr := startGateServer(t, provider)

		linker := NewGateLinker(context.Background(), baseLinkOptions())
		linker.dispatcher.ReplaceServices(gateService("gate-1", addr))

		total, err := linker.Broadcast(context.Background(), &BroadcastArgs{Kind: session.Conn, Message: msg, Ack: true})
		if err != nil {
			t.Fatalf("Broadcast failed: %v", err)
		}
		if total != 3 {
			t.Errorf("total = %d, want 3", total)
		}
	})

	t.Run("pack error", func(t *testing.T) {
		opts := baseLinkOptions()
		opts.Codec = &codecStub{marshalFn: func(any) ([]byte, error) { return nil, errors.ErrInvalidFormat }}

		linker := NewGateLinker(context.Background(), opts)
		linker.dispatcher.ReplaceServices(gateService("gate-1", "127.0.0.1:1"))

		_, err := linker.Broadcast(context.Background(), &BroadcastArgs{Kind: session.Conn, Message: &Message{Route: 1, Data: struct{}{}}})
		if !errors.Is(err, errors.ErrInvalidFormat) {
			t.Errorf("expect ErrInvalidFormat, got %v", err)
		}
	})

	t.Run("build error", func(t *testing.T) {
		linker := NewGateLinker(context.Background(), baseLinkOptions())
		linker.dispatcher.ReplaceServices(gateService("bad", "127.0.0.1:99999"))

		_, err := linker.Broadcast(context.Background(), &BroadcastArgs{Kind: session.Conn, Message: msg, Ack: true})
		if err == nil {
			t.Errorf("expect a build error for an invalid port")
		}
	})

	// With several endpoints the fan-out goroutines share one packed buffer. Failing builds
	// short-circuit before the buffer is encoded, which covers the fan-out error branch;
	// TestGateLinkerBroadcastFanOut covers the live multi-gate broadcast.
	t.Run("fan-out build error", func(t *testing.T) {
		linker := NewGateLinker(context.Background(), baseLinkOptions())
		linker.dispatcher.ReplaceServices(gateService("bad-1", "127.0.0.1:99999"), gateService("bad-2", "127.0.0.1:99999"))

		_, err := linker.Broadcast(context.Background(), &BroadcastArgs{Kind: session.Conn, Message: msg, Ack: true})
		if err == nil {
			t.Errorf("expect a build error for invalid ports")
		}
	})
}

// TestGateLinkerPublish verifies publishing over zero, one and many gate endpoints.
func TestGateLinkerPublish(t *testing.T) {
	msg := &Message{Route: 1, Data: []byte("hello")}

	t.Run("no endpoints", func(t *testing.T) {
		linker := NewGateLinker(context.Background(), baseLinkOptions())

		total, err := linker.Publish(context.Background(), &PublishArgs{Channel: "room", Message: msg})
		if err != nil || total != 0 {
			t.Errorf("Publish = (%d, %v), want (0, nil)", total, err)
		}
	})

	t.Run("one gate", func(t *testing.T) {
		provider := &gateProviderStub{}
		_, addr := startGateServer(t, provider)

		linker := NewGateLinker(context.Background(), baseLinkOptions())
		linker.dispatcher.ReplaceServices(gateService("gate-1", addr))

		total, err := linker.Publish(context.Background(), &PublishArgs{Channel: "room", Message: msg, Ack: true})
		if err != nil {
			t.Fatalf("Publish failed: %v", err)
		}
		if total != 2 {
			t.Errorf("total = %d, want 2", total)
		}
	})

	t.Run("pack error", func(t *testing.T) {
		opts := baseLinkOptions()
		opts.Codec = &codecStub{marshalFn: func(any) ([]byte, error) { return nil, errors.ErrInvalidFormat }}

		linker := NewGateLinker(context.Background(), opts)
		linker.dispatcher.ReplaceServices(gateService("gate-1", "127.0.0.1:1"))

		_, err := linker.Publish(context.Background(), &PublishArgs{Channel: "room", Message: &Message{Route: 1, Data: struct{}{}}})
		if !errors.Is(err, errors.ErrInvalidFormat) {
			t.Errorf("expect ErrInvalidFormat, got %v", err)
		}
	})

	t.Run("build error", func(t *testing.T) {
		linker := NewGateLinker(context.Background(), baseLinkOptions())
		linker.dispatcher.ReplaceServices(gateService("bad", "127.0.0.1:99999"))

		_, err := linker.Publish(context.Background(), &PublishArgs{Channel: "room", Message: msg, Ack: true})
		if err == nil {
			t.Errorf("expect a build error for an invalid port")
		}
	})

	// See the broadcast fan-out case: failing builds cover the multi-endpoint error branch, while
	// TestGateLinkerPublishFanOut covers the live multi-gate publish.
	t.Run("fan-out build error", func(t *testing.T) {
		linker := NewGateLinker(context.Background(), baseLinkOptions())
		linker.dispatcher.ReplaceServices(gateService("bad-1", "127.0.0.1:99999"), gateService("bad-2", "127.0.0.1:99999"))

		_, err := linker.Publish(context.Background(), &PublishArgs{Channel: "room", Message: msg, Ack: true})
		if err == nil {
			t.Errorf("expect a build error for invalid ports")
		}
	})
}

// TestGateLinkerPackMessage verifies message and buffer packing, including encryption and errors.
func TestGateLinkerPackMessage(t *testing.T) {
	t.Run("buffer variants", func(t *testing.T) {
		opts := baseLinkOptions()
		opts.Encryptor = encryptorStub{}
		linker := NewGateLinker(context.Background(), opts)

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
				data, err := linker.PackBuffer(tt.message, tt.encrypt)
				if err != nil {
					t.Fatalf("PackBuffer failed: %v", err)
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
		linker := NewGateLinker(context.Background(), opts)

		if _, err := linker.PackBuffer(struct{}{}, false); !errors.Is(err, errors.ErrInvalidFormat) {
			t.Errorf("expect ErrInvalidFormat, got %v", err)
		}
	})

	t.Run("pack message", func(t *testing.T) {
		linker := NewGateLinker(context.Background(), baseLinkOptions())

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
		linker := NewGateLinker(context.Background(), opts)

		if _, err := linker.PackMessage(&Message{Route: 1, Data: struct{}{}}, false); !errors.Is(err, errors.ErrInvalidFormat) {
			t.Errorf("expect ErrInvalidFormat, got %v", err)
		}
	})
}

// TestGateLinkerWatchUserLocate verifies the locate event watcher cache maintenance.
func TestGateLinkerWatchUserLocate(t *testing.T) {
	t.Run("no locator", func(t *testing.T) {
		linker := NewGateLinker(context.Background(), baseLinkOptions())
		linker.WatchUserLocate()

		if _, ok := linker.sources.Load(int64(1)); ok {
			t.Errorf("expect no sources without a locator")
		}
	})

	t.Run("events", func(t *testing.T) {
		watcher := &locateWatcherStub{steps: make(chan locateWatchStep, 6)}
		watcher.steps <- locateWatchStep{err: errors.ErrUnknownError}
		watcher.steps <- locateWatchStep{events: []*locate.Event{{UID: 1, Type: locate.BindGate, InsID: "gate-1"}}}
		watcher.steps <- locateWatchStep{events: []*locate.Event{{UID: 3, Type: locate.BindGate, InsID: "gate-3"}}}
		watcher.steps <- locateWatchStep{events: []*locate.Event{{UID: 1, Type: locate.UnbindGate, InsID: "gate-1"}}}
		watcher.steps <- locateWatchStep{events: []*locate.Event{{UID: 4, Type: locate.EventType(0), InsID: "ignored"}}}
		watcher.steps <- locateWatchStep{err: errors.ErrWatcherStopped}

		opts := baseLinkOptions()
		opts.Locator = &locatorStub{watchFn: func(context.Context, ...string) (locate.Watcher, error) {
			return watcher, nil
		}}

		linker := NewGateLinker(context.Background(), opts)
		linker.WatchUserLocate()

		if !waitFor(t, 2*time.Second, func() bool {
			gid, ok := linker.sources.Load(int64(3))
			return ok && gid.(string) == "gate-3"
		}) {
			t.Fatalf("expect uid 3 to be cached on gate-3")
		}

		if !waitFor(t, 2*time.Second, func() bool {
			_, ok := linker.sources.Load(int64(1))
			return !ok
		}) {
			t.Errorf("expect uid 1 to be removed by the unbind event")
		}

		if _, ok := linker.sources.Load(int64(4)); ok {
			t.Errorf("expect the unknown event type to be ignored")
		}
	})
}

// TestGateLinkerWatchClusterInstance verifies the cluster instance watcher dispatcher refresh.
func TestGateLinkerWatchClusterInstance(t *testing.T) {
	watcher := &registryWatcherStub{steps: make(chan registryWatchStep, 4)}
	watcher.steps <- registryWatchStep{err: errors.ErrUnknownError}
	watcher.steps <- registryWatchStep{services: []*registry.ServiceInstance{gateService("gate-1", "127.0.0.1:1")}}
	watcher.steps <- registryWatchStep{services: []*registry.ServiceInstance{gateService("gate-2", "127.0.0.1:2")}}
	watcher.steps <- registryWatchStep{err: context.Canceled}

	opts := baseLinkOptions()
	opts.Registry = &registryStub{watchFn: func(context.Context, string) (registry.Watcher, error) {
		return watcher, nil
	}}

	linker := NewGateLinker(context.Background(), opts)
	linker.WatchClusterInstance()

	if !waitFor(t, 2*time.Second, func() bool { return linker.HasGate("gate-2") }) {
		t.Fatalf("expect gate-2 to be registered")
	}
	if linker.HasGate("gate-1") {
		t.Errorf("expect gate-1 to be replaced by the newest service list")
	}
}
