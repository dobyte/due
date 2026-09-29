package queue_test

import (
	"sync"
	"testing"
	"time"

	"github.com/dobyte/due/v2/core/queue"
	"github.com/dobyte/due/v2/errors"
)

// waitSignal 等待信号，超时则判定测试失败
// @param t *testing.T 测试上下文
// @param sig <-chan struct{} 待等待的信号通道
// @param msg string 超时时的失败信息
func waitSignal(t *testing.T, sig <-chan struct{}, msg string) {
	t.Helper()

	select {
	case <-sig:
	case <-time.After(time.Second):
		t.Fatal(msg)
	}
}

// TestQueue_WriteAndRead 校验队列的写入、读取与数据处理确认
func TestQueue_WriteAndRead(t *testing.T) {
	q := queue.NewQueue[int](4, 0)

	for i := 0; i < 4; i++ {
		if err := q.Write(i); err != nil {
			t.Fatalf("write data failed, err: %v", err)
		}
	}

	for i := 0; i < 4; i++ {
		v, ok := <-q.Read()
		if !ok {
			t.Fatal("the queue channel is closed unexpectedly")
		}

		if v != i {
			t.Fatalf("the data is out of order, expect: %d, actual: %d", i, v)
		}

		q.Done(false)
	}

	q.Close()

	sig := make(chan struct{})
	go func() {
		q.Wait()
		close(sig)
	}()

	waitSignal(t, sig, "wait did not return after the queue is closed")
}

// TestQueue_WriteWithExternalLock 校验复用调用方读写锁时的写入与关闭
func TestQueue_WriteWithExternalLock(t *testing.T) {
	q := queue.NewQueue[int](2, 0, &sync.RWMutex{})

	if err := q.Write(1); err != nil {
		t.Fatalf("write data failed, err: %v", err)
	}

	v, ok := <-q.Read()
	if !ok {
		t.Fatal("the queue channel is closed unexpectedly")
	}

	if v != 1 {
		t.Fatalf("the data is out of order, expect: 1, actual: %d", v)
	}

	q.Done(false)
	q.Close()
}

// TestQueue_WriteTimeout 校验队列写满后非阻塞写入的超时行为
func TestQueue_WriteTimeout(t *testing.T) {
	q := queue.NewQueue[int](1, 10*time.Millisecond)

	if err := q.Write(1); err != nil {
		t.Fatalf("write data failed, err: %v", err)
	}

	if err := q.Write(2); !errors.Is(err, errors.ErrWriteTimeout) {
		t.Fatalf("write data expect ErrWriteTimeout, actual: %v", err)
	}

	q.Close()
}

// TestQueue_WriteBlock 校验阻塞写入不受超时限制，且会在消费方腾出空间后写入成功
func TestQueue_WriteBlock(t *testing.T) {
	q := queue.NewQueue[int](1, 10*time.Millisecond)

	if err := q.Write(1); err != nil {
		t.Fatalf("write data failed, err: %v", err)
	}

	sig := make(chan struct{})

	go func() {
		if err := q.Write(2, true); err != nil {
			t.Errorf("write data failed, err: %v", err)
		}

		close(sig)
	}()

	select {
	case <-sig:
		t.Fatal("the blocking write did not wait for the free space")
	case <-time.After(50 * time.Millisecond):
	}

	if v, _ := <-q.Read(); v != 1 {
		t.Fatalf("the data is out of order, expect: 1, actual: %d", v)
	}

	q.Done(false)

	waitSignal(t, sig, "the blocking write did not return after the free space")

	if v, _ := <-q.Read(); v != 2 {
		t.Fatalf("the data is out of order, expect: 2, actual: %d", v)
	}

	q.Done(false)
	q.Close()
}

// TestQueue_Hang 校验消费到结束信号后队列挂起并释放等待
func TestQueue_Hang(t *testing.T) {
	q := queue.NewQueue[any](4, 0)

	if err := q.Write(nil); err != nil {
		t.Fatalf("write sentinel data failed, err: %v", err)
	}

	v, ok := <-q.Read()
	if !ok {
		t.Fatal("the queue channel is closed unexpectedly")
	}

	q.Done(v == nil)

	sig := make(chan struct{})
	go func() {
		q.Wait()
		close(sig)
	}()

	waitSignal(t, sig, "wait did not return after the sentinel data is handled")

	if err := q.Write(1); !errors.Is(err, errors.ErrQueueHanged) {
		t.Fatalf("write data expect ErrQueueHanged, actual: %v", err)
	}

	q.Close()
}

// TestQueue_Closed 校验队列关闭后的写入拦截、重复关闭与残留数据清理
func TestQueue_Closed(t *testing.T) {
	q := queue.NewQueue[int](2, 0)

	if err := q.Write(1); err != nil {
		t.Fatalf("write data failed, err: %v", err)
	}

	q.Close()
	q.Close()

	if err := q.Write(2); !errors.Is(err, errors.ErrQueueClosed) {
		t.Fatalf("write data expect ErrQueueClosed, actual: %v", err)
	}

	var count int

	q.Clean(func(int) { count++ })

	if count != 1 {
		t.Fatalf("clean residual data failed, expect: 1, actual: %d", count)
	}
}

// TestTasker_CommitAndHandle 校验任务的提交、处理与等待组释放
func TestTasker_CommitAndHandle(t *testing.T) {
	tk := queue.NewTasker(4, 0)

	executed := make(chan struct{}, 2)

	wg, err := tk.Commit(func() { executed <- struct{}{} }, true)
	if err != nil {
		t.Fatalf("commit task failed, err: %v", err)
	}

	if wg == nil {
		t.Fatal("the wait group is nil in the waiting mode")
	}

	tk.Handle(<-tk.Read(), true)

	waitSignal(t, executed, "the task is not executed")

	sig := make(chan struct{})
	go func() {
		wg.Wait()
		close(sig)
	}()

	waitSignal(t, sig, "the wait group is not released after the task is handled")

	wg2, err := tk.Commit(func() { executed <- struct{}{} })
	if err != nil {
		t.Fatalf("commit task failed, err: %v", err)
	}

	if wg2 != nil {
		t.Fatal("the wait group is not nil in the non-waiting mode")
	}

	tk.Handle(<-tk.Read(), true)

	waitSignal(t, executed, "the task is not executed")

	tk.Close()
}

// TestTasker_Done 校验结束信号触发队列挂起并拦截后续任务提交
func TestTasker_Done(t *testing.T) {
	tk := queue.NewTasker(4, 0)

	if err := tk.Done(); err != nil {
		t.Fatalf("done failed, err: %v", err)
	}

	tk.Handle(<-tk.Read(), true)

	sig := make(chan struct{})
	go func() {
		tk.Wait()
		close(sig)
	}()

	waitSignal(t, sig, "wait did not return after the sentinel task is handled")

	if _, err := tk.Commit(func() {}); !errors.Is(err, errors.ErrQueueHanged) {
		t.Fatalf("commit task expect ErrQueueHanged, actual: %v", err)
	}

	tk.Close()
}

// TestTasker_Clean 校验未完成的任务被清理后等待组同步释放
func TestTasker_Clean(t *testing.T) {
	tk := queue.NewTasker(4, 0)

	wg, err := tk.Commit(func() { t.Error("the residual task should not be executed") }, true)
	if err != nil {
		t.Fatalf("commit task failed, err: %v", err)
	}

	if wg == nil {
		t.Fatal("the wait group is nil in the waiting mode")
	}

	tk.Close()
	tk.Clean()

	sig := make(chan struct{})
	go func() {
		wg.Wait()
		close(sig)
	}()

	waitSignal(t, sig, "the wait group is not released after the task is cleaned")
}
