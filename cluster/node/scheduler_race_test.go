package node

import (
	"os"
	"os/exec"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"testing"
)

// TestSchedulerRemoveConcurrentBindRepro 在子进程中运行工作函数，
// 因为存在缺陷的调度器会触发运行时致命错误，直接终止整个进程，而不是返回错误。
func TestSchedulerRemoveConcurrentBindRepro(t *testing.T) {
	if os.Getenv("DUE_SCHEDULER_RACE_WORKER") == "1" {
		schedulerRemoveConcurrentBindWorker(t)
		return
	}
	if os.Getenv("DUE_SCHEDULER_RACE") != "1" {
		t.Skip("set DUE_SCHEDULER_RACE=1 to run the scheduler race reproducer")
	}

	cmd := exec.Command(os.Args[0], "-test.run=^TestSchedulerRemoveConcurrentBindRepro$", "-test.v")
	cmd.Env = append(os.Environ(), "DUE_SCHEDULER_RACE_WORKER=1")
	output, err := cmd.CombinedOutput()
	if err == nil {
		t.Fatalf("scheduler race reproducer completed without crashing; output:\n%s", output)
	}
	if !strings.Contains(string(output), "concurrent map iteration and map write") {
		t.Fatalf("worker exited without the expected runtime fatal: %v\noutput:\n%s", err, output)
	}
	t.Logf("reproduced scheduler race: %s", strings.TrimSpace(string(output)))
}

// TestSchedulerRemoveConcurrentBind 是修复后调度器的回归测试。
// 测试始终运行相同的并发负载，并要求工作进程正常退出。
func TestSchedulerRemoveConcurrentBind(t *testing.T) {
	if os.Getenv("DUE_SCHEDULER_RACE_WORKER") == "1" {
		schedulerRemoveConcurrentBindWorker(t)
		return
	}

	cmd := exec.Command(os.Args[0], "-test.run=^TestSchedulerRemoveConcurrentBind$", "-test.v")
	cmd.Env = append(os.Environ(), "DUE_SCHEDULER_RACE_WORKER=1")
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("scheduler concurrent bind regression failed: %v\noutput:\n%s", err, output)
	}
}

func schedulerRemoveConcurrentBindWorker(t *testing.T) {
	const (
		actorKind  = "scheduler-race"
		actorCount = 8
	)
	const seedEntries int64 = 200_000
	const bindOps = 20_000

	proxy := NewNode(WithID("scheduler-race-node")).Proxy()
	for i := 0; i < actorCount; i++ {
		if _, err := proxy.Spawn(
			func(*Actor, ...any) Processor { return &BaseProcessor{} },
			WithActorKind(actorKind),
			WithActorID(strconv.Itoa(i)),
		); err != nil {
			t.Fatalf("spawn actor %d: %v", i, err)
		}
	}

	// 增大关系表遍历时间，确保它有机会与并发的 BindActor 写入重叠。
	for uid := int64(1); uid <= seedEntries; uid++ {
		if err := proxy.BindActor(uid, actorKind, "1"); err != nil {
			t.Fatalf("seed relation uid=%d: %v", uid, err)
		}
	}

	workers := runtime.GOMAXPROCS(0) * 4
	if workers < 8 {
		workers = 8
	}
	start := make(chan struct{})
	ready := make(chan struct{}, workers)
	var wg sync.WaitGroup
	for worker := 0; worker < workers; worker++ {
		worker := worker
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			for op := 0; op < bindOps; op++ {
				uid := seedEntries + int64(worker*bindOps+op) + 1
				target := strconv.Itoa(op%(actorCount-1) + 1)
				if err := proxy.BindActor(uid, actorKind, target); err == nil && op == 0 {
					ready <- struct{}{}
				}
			}
		}()
	}

	close(start)
	for i := 0; i < workers; i++ {
		<-ready
	}

	proxy.Kill(actorKind, "0")
	wg.Wait()
}
