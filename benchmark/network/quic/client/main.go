package main

import (
	"fmt"
	"math"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/dobyte/due/network/quic/v2"
	"github.com/dobyte/due/v2/core/buffer"
	"github.com/dobyte/due/v2/log"
	"github.com/dobyte/due/v2/network"
	"github.com/dobyte/due/v2/packet"
	"github.com/dobyte/due/v2/utils/xrand"
)

func main() {
	samples := []struct {
		c    int // 并发数
		n    int // 请求数
		size int // 数据包大小
	}{
		{
			c:    50,
			n:    1000000,
			size: 128,
		},
		{
			c:    50,
			n:    1000000,
			size: 1024,
		},
		{
			c:    100,
			n:    1000000,
			size: 1024,
		},
		{
			c:    200,
			n:    1000000,
			size: 1024,
		},
		{
			c:    300,
			n:    1000000,
			size: 1024,
		},
		{
			c:    400,
			n:    1000000,
			size: 1024,
		},
		{
			c:    500,
			n:    1000000,
			size: 1024,
		},
		{
			c:    1000,
			n:    1000000,
			size: 2 * 1024,
		},
	}

	for _, sample := range samples {
		doPressureTest(sample.c, sample.n, sample.size)
	}
}

// 执行压力测试
func doPressureTest(c int, n int, size int) {
	var (
		wg            sync.WaitGroup
		connMu        sync.Mutex
		totalSent     int32                // 原子递增的 Seq 生成器，最终值即为发送总数
		totalRecv     int64                // 接收总数
		latencyIdx    int64                // latencies 切片的原子写入下标
		dialErrors    int64                // 拨号失败次数
		pushErrors    int64                // push 失败次数
		sendTimes     = make([]int64, n+1) // 按 Seq 索引存储发送时间戳 (UnixNano), Seq 从1开始
		latencies     = make([]float64, n) // 预分配，收集延迟值 (ms)
		connDurations []float64            // 每个连接的建立耗时 (ms)

		client = quic.NewClient(quic.WithClientHeartbeatInterval(0), quic.WithClientCredentials("../certs/cert.pem", "localhost"))
	)

	client.OnReceive(func(conn network.Conn, buf buffer.Buffer) {
		defer buf.Release()

		_, seq, _, err := packet.UnpackMessage(buf)
		if err != nil {
			return
		}

		// 利用 Seq 配对计算单次请求 RTT
		sendTime := sendTimes[seq]
		if sendTime > 0 {
			lat := float64(time.Now().UnixNano()-sendTime) / 1e6 // ns -> ms
			idx := atomic.AddInt64(&latencyIdx, 1) - 1
			latencies[idx] = lat
		}

		atomic.AddInt64(&totalRecv, 1)
		wg.Done()
	})

	buffer := []byte(xrand.Letters(size))
	chMsg := make(chan struct{}, n)

	// 建立连接，记录连接耗时
	for i := 0; i < c; i++ {
		dialStart := time.Now()
		conn, err := client.Dial()
		if err != nil {
			log.Errorf("client dial failed: %v", err)
			atomic.AddInt64(&dialErrors, 1)
			i--
			continue
		}

		connMu.Lock()
		connDurations = append(connDurations, float64(time.Since(dialStart))/float64(time.Millisecond))
		connMu.Unlock()

		go func(conn network.Conn) {
			defer conn.Close(true)

			for range chMsg {
				seq := atomic.AddInt32(&totalSent, 1)
				sendTimes[seq] = time.Now().UnixNano()

				msg, err := packet.PackMessage(&packet.Message{
					Seq:    seq,
					Route:  1,
					Buffer: buffer,
				})
				if err != nil {
					log.Errorf("pack message failed: %v", err)
					atomic.AddInt64(&pushErrors, 1)
					wg.Done()
					return
				}

				if err = conn.Push(msg); err != nil {
					log.Errorf("push message failed: %v", err)
					atomic.AddInt64(&pushErrors, 1)
					wg.Done()
					return
				}
			}
		}(conn)
	}

	wg.Add(n)

	startTime := time.Now().UnixNano()

	for range n {
		chMsg <- struct{}{}
	}

	wg.Wait()

	close(chMsg)

	duration := float64(time.Now().UnixNano()-startTime) / float64(time.Second)
	recv := atomic.LoadInt64(&totalRecv)
	dialErr := atomic.LoadInt64(&dialErrors)
	pushErr := atomic.LoadInt64(&pushErrors)

	// 延迟统计
	latencies = latencies[:recv] // 截取有效数据
	sort.Float64s(latencies)
	latMin, latMax, latAvg, latStd := latencyStats(latencies)
	latP50 := percentile(latencies, 0.50)
	latP75 := percentile(latencies, 0.75)
	latP90 := percentile(latencies, 0.90)
	latP95 := percentile(latencies, 0.95)
	latP99 := percentile(latencies, 0.99)
	latP999 := percentile(latencies, 0.999)

	// 连接耗时统计
	connMin, connAvg, connMax := connStats(connDurations)

	// 吞吐量统计
	tps := int64(float64(recv) / duration)
	bandwidth := float64(recv*int64(size)*2) / duration / (1024 * 1024) // MB/s (收发双向)

	// 成功率
	successRate := float64(0)
	if sent := int32(totalSent); sent > 0 {
		successRate = float64(recv) / float64(sent) * 100
	}

	// 输出结果
	fmt.Printf("\n")
	fmt.Printf("%s\n", sectionLineWith("QUIC BENCHMARK", '='))
	fmt.Printf("  Concurrency: %-5d ｜ Requests: %-9d ｜ Size: %s\n",
		c, n, convBytes(size))
	fmt.Printf("================================================================\n")
	fmt.Printf("  Protocol:            quic\n")
	fmt.Printf("  Target:              127.0.0.1:3553\n")
	fmt.Printf("  Duration:            %.3fs\n", duration)
	fmt.Printf("%s\n", sectionLine("Throughput"))
	fmt.Printf("  TPS:                 %s req/s\n", formatNum(tps))
	fmt.Printf("  Bandwidth:           %.2f MB/s\n", bandwidth)
	fmt.Printf("%s\n", sectionLine("Latency (ms)"))
	fmt.Printf("  Min:                 %.3f\n", latMin)
	fmt.Printf("  Max:                 %.3f\n", latMax)
	fmt.Printf("  Avg:                 %.3f\n", latAvg)
	fmt.Printf("  StdDev:              %.3f\n", latStd)
	fmt.Printf("  P50:                 %.3f\n", latP50)
	fmt.Printf("  P75:                 %.3f\n", latP75)
	fmt.Printf("  P90:                 %.3f\n", latP90)
	fmt.Printf("  P95:                 %.3f\n", latP95)
	fmt.Printf("  P99:                 %.3f\n", latP99)
	fmt.Printf("  P999:                %.3f\n", latP999)
	fmt.Printf("%s\n", sectionLine("Reliability"))
	fmt.Printf("  Success Rate:        %.2f%%\n", successRate)
	fmt.Printf("  Sent:                %s\n", formatNum(int64(totalSent)))
	fmt.Printf("  Recv:                %s\n", formatNum(recv))
	fmt.Printf("  Dial Errors:         %d\n", dialErr)
	fmt.Printf("  Push Errors:         %d\n", pushErr)
	fmt.Printf("%s\n", sectionLine("Connection"))
	fmt.Printf("  Connections:         %d\n", len(connDurations))
	fmt.Printf("  Conn Avg (ms):       %.3f\n", connAvg)
	fmt.Printf("  Conn Max (ms):       %.3f\n", connMax)
	if connMin >= 0 {
		fmt.Printf("  Conn Min (ms):       %.3f\n", connMin)
	}
	fmt.Printf("%s\n\n", sectionLineWith("END", '='))
}

// sectionLine 生成中间带标题的分隔线，标题居中
func sectionLine(title string) string {
	return sectionLineWith(title, '-')
}

// sectionLineWith 生成中间带标题的分隔线，可指定填充字符
func sectionLineWith(title string, ch byte) string {
	const width = 64
	label := " " + title + " "
	n := width - len(label)
	left := n / 2
	right := n - left
	return strings.Repeat(string(ch), left) + label + strings.Repeat(string(ch), right)
}

// latencyStats 计算延迟的最小/最大/平均/标准差
func latencyStats(data []float64) (min, max, avg, std float64) {
	if len(data) == 0 {
		return 0, 0, 0, 0
	}

	min, max = data[0], data[0]
	sum := 0.0
	for _, v := range data {
		sum += v
		if v < min {
			min = v
		}
		if v > max {
			max = v
		}
	}

	avg = sum / float64(len(data))

	sumSq := 0.0
	for _, v := range data {
		sumSq += (v - avg) * (v - avg)
	}
	std = math.Sqrt(sumSq / float64(len(data)))

	return
}

// percentile 计算分位值，使用线性插值
func percentile(data []float64, p float64) float64 {
	if len(data) == 0 {
		return 0
	}
	if p <= 0 {
		return data[0]
	}
	if p >= 1 {
		return data[len(data)-1]
	}

	pos := p * float64(len(data)-1)
	lower := int(math.Floor(pos))
	upper := int(math.Ceil(pos))
	if lower == upper {
		return data[lower]
	}

	frac := pos - float64(lower)
	return data[lower]*(1-frac) + data[upper]*frac
}

// connStats 计算连接耗时的最小/平均/最大
func connStats(durations []float64) (min, avg, max float64) {
	if len(durations) == 0 {
		return 0, 0, 0
	}

	min, max = durations[0], durations[0]
	sum := 0.0
	for _, d := range durations {
		sum += d
		if d < min {
			min = d
		}
		if d > max {
			max = d
		}
	}

	return min, sum / float64(len(durations)), max
}

// formatNum 格式化大数字，添加千分位分隔符
func formatNum(n int64) string {
	if n < 1000 {
		return fmt.Sprintf("%d", n)
	}

	s := fmt.Sprintf("%d", n)
	var result []byte
	for i := len(s); i > 0; i -= 3 {
		start := i - 3
		if start < 0 {
			start = 0
		}
		if len(result) > 0 {
			result = append([]byte(s[start:i]+","), result...)
		} else {
			result = []byte(s[start:i])
		}
	}
	return string(result)
}

func convBytes(bytes int) string {
	const (
		KB = 1024
		MB = 1024 * KB
		GB = 1024 * MB
		TB = 1024 * GB
	)

	switch {
	case bytes < KB:
		return fmt.Sprintf("%.2fB", float64(bytes))
	case bytes < MB:
		return fmt.Sprintf("%.2fKB", float64(bytes)/KB)
	case bytes < GB:
		return fmt.Sprintf("%.2fMB", float64(bytes)/MB)
	case bytes < TB:
		return fmt.Sprintf("%.2fGB", float64(bytes)/GB)
	default:
		return fmt.Sprintf("%.2fTB", float64(bytes)/TB)
	}
}
