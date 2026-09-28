package node

import (
	"sync"
	"sync/atomic"
	"testing"

	"github.com/dobyte/due/v2/errors"
)

// newTestScheduler 创建用于测试的调度器（绑定关系操作不依赖Node实例）
// @return @1 *Scheduler 调度器实例
func newTestScheduler() *Scheduler {
	return newScheduler(nil)
}

// registerTestActor 直接向调度器注册一个处于启动状态的测试Actor
// @param s *Scheduler 调度器
// @param kind string Actor类型
// @param id string Actor编号
// @return @1 *Actor 测试Actor实例
func registerTestActor(s *Scheduler, kind, id string) *Actor {
	act := &Actor{
		opts:      &actorOptions{kind: kind, id: id},
		scheduler: s,
	}
	act.pid = kind + "/" + id
	act.state.Store(started)
	s.actors.Store(act.PID(), act)
	return act
}

func TestSchedulerBindAndLoadActor(t *testing.T) {
	s := newTestScheduler()
	registerTestActor(s, "room", "1")

	if err := s.bindActor(1001, "room", "1"); err != nil {
		t.Fatalf("bindActor failed: %v", err)
	}

	act, ok := s.loadActor(1001, "room")
	if !ok {
		t.Fatal("loadActor: actor not found")
	}
	if act.Kind() != "room" || act.ID() != "1" {
		t.Fatalf("loadActor: unexpected actor %s", act.PID())
	}

	if err := s.unbindActor(1001, "room"); err != nil {
		t.Fatalf("unbindActor failed: %v", err)
	}

	if _, ok = s.loadActor(1001, "room"); ok {
		t.Fatal("loadActor: actor should be unbound")
	}
}

func TestSchedulerBindActorFailed(t *testing.T) {
	s := newTestScheduler()

	if err := s.bindActor(0, "room", "1"); !errors.Is(err, errors.ErrIllegalOperation) {
		t.Fatalf("expect ErrIllegalOperation, got %v", err)
	}

	if err := s.bindActor(1001, "room", "1"); !errors.Is(err, errors.ErrNotFoundActor) {
		t.Fatalf("expect ErrNotFoundActor, got %v", err)
	}

	act := registerTestActor(s, "room", "1")
	act.state.Store(destroyed)

	if err := s.bindActor(1001, "room", "1"); !errors.Is(err, errors.ErrActorNotStarted) {
		t.Fatalf("expect ErrActorNotStarted, got %v", err)
	}
}

func TestSchedulerUnbindAllActor(t *testing.T) {
	s := newTestScheduler()
	act := registerTestActor(s, "room", "1")

	const uids = 1000
	for i := 0; i < uids; i++ {
		if err := s.bindActor(int64(i+1), "room", "1"); err != nil {
			t.Fatalf("bindActor failed: %v", err)
		}
	}

	s.unbindAllActor(act)

	for i := 0; i < uids; i++ {
		if _, ok := s.loadActor(int64(i+1), "room"); ok {
			t.Fatalf("uid %d should be unbound", i+1)
		}
	}

	total := 0
	for i := range s.shards {
		total += len(s.shards[i].relations)
	}
	if total != 0 {
		t.Fatalf("relations should be empty, got %d", total)
	}
}

func TestSchedulerConcurrentBindUnbind(t *testing.T) {
	s := newTestScheduler()
	registerTestActor(s, "room", "1")

	var wg sync.WaitGroup
	for g := 0; g < 8; g++ {
		wg.Add(1)
		go func(g int) {
			defer wg.Done()
			base := int64(g*10000) + 1
			for i := 0; i < 5000; i++ {
				uid := base + int64(i)
				if err := s.bindActor(uid, "room", "1"); err != nil {
					t.Errorf("bindActor failed: %v", err)
					return
				}
				if _, ok := s.loadActor(uid, "room"); !ok {
					t.Errorf("loadActor: actor not found, uid: %d", uid)
					return
				}
				if err := s.unbindActor(uid, "room"); err != nil {
					t.Errorf("unbindActor failed: %v", err)
					return
				}
			}
		}(g)
	}
	wg.Wait()
}

// TestSchedulerBindActorRaceDestroy 绑定与销毁并发：销毁完成后调度器中不得残留指向已销毁Actor的绑定关系
func TestSchedulerBindActorRaceDestroy(t *testing.T) {
	for i := 0; i < 100; i++ {
		s := newTestScheduler()
		act := registerTestActor(s, "room", "1")

		var wg sync.WaitGroup
		wg.Add(2)

		go func() {
			defer wg.Done()
			_ = s.bindActor(1001, "room", "1")
		}()

		go func() {
			defer wg.Done()
			act.state.CompareAndSwap(started, destroyed)
			s.unbindAllActor(act)
		}()

		wg.Wait()

		if _, ok := s.loadActor(1001, "room"); ok {
			t.Fatal("relations should not contain destroyed actor")
		}
	}
}

// BenchmarkSchedulerLoadActor 模拟真实负载：99%读取绑定关系 + 1%绑定/解绑
func BenchmarkSchedulerLoadActor(b *testing.B) {
	s := newTestScheduler()
	registerTestActor(s, "room", "1")

	const uids = 100000
	for i := 0; i < uids; i++ {
		if err := s.bindActor(int64(i)+1, "room", "1"); err != nil {
			b.Fatalf("bindActor failed: %v", err)
		}
	}

	var n atomic.Int64
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			seq := n.Add(1)
			uid := seq%uids + 1
			if seq%100 == 0 {
				_ = s.unbindActor(uid, "room")
				_ = s.bindActor(uid, "room", "1")
			} else {
				_, _ = s.loadActor(uid, "room")
			}
		}
	})
}
