package main

import (
	"fmt"
	"math"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/dobyte/due/network/ws/v2"
	"github.com/dobyte/due/v2"
	"github.com/dobyte/due/v2/cluster"
	"github.com/dobyte/due/v2/cluster/client"
	"github.com/dobyte/due/v2/log"
	"github.com/dobyte/due/v2/utils/xrand"
)

const greet = 1

var (
	wg            *sync.WaitGroup
	startTime     int64
	totalSent     int64
	totalRecv     int64
	message       string
	sendTimes     []int64   // 按 Seq 索引存储发送时间戳 (UnixNano), Seq 从1开始
	latencies     []float64 // 收集延迟值 (ms)
	latencyIdx    int64     // latencies 切片的原子写入下标
	dialErrors    int64     // 拨号失败次数
	pushErrors    int64     // push 失败次数
	connDurations []float64 // 每个连接的建立耗时 (ms)
	connMu        sync.Mutex
)

type greetReq struct {
	Message string `json:"message"`
}

type greetRes struct {
	Message string `json:"message"`
}

func main() {
	// 创建容器
	container := due.NewContainer()
	// 创建客户端组件
	component := client.NewClient(
		client.WithClient(ws.NewClient()),
	)
	// 初始化监听
	initListen(component.Proxy())
	// 添加客户端组件
	container.Add(component)
	// 启动容器
	container.Serve(true)
}

// 初始化监听
func initListen(proxy *client.Proxy) {
	// 监听组件启动
	proxy.AddHookListener(cluster.Start, startHandler)
	// 监听消息回复
	proxy.AddRouteHandler(greet, greetHandler)
}

// 组件启动处理器
func startHandler(proxy *client.Proxy) {
	samples := []struct {
		c    int // 并发数
		n    int // 请求数
		size int // 数据包大小
	}{
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
			size: 1024,
		},
		{
			c:    1000,
			n:    1000000,
			size: 2 * 1024,
		},
	}

	for _, sample := range samples {
		doPressureTest(proxy, sample.c, sample.n, sample.size)
	}
}

// 消息回复处理器
func greetHandler(ctx *client.Context) {
	res := &greetRes{}

	if err := ctx.Parse(res); err != nil {
		log.Errorf("invalid response message, err: %v", err)
		return
	}

	// 利用 Seq 配对计算单次请求 RTT
	seq := ctx.Seq()
	sendTime := sendTimes[seq]
	if sendTime > 0 {
		lat := float64(time.Now().UnixNano()-sendTime) / 1e6 // ns -> ms
		idx := atomic.AddInt64(&latencyIdx, 1) - 1
		latencies[idx] = lat
	}

	atomic.AddInt64(&totalRecv, 1)

	wg.Done()
}

// 执行压力测试
func doPressureTest(proxy *client.Proxy, c, n, size int) {
	wg = &sync.WaitGroup{}
	message = xrand.Letters(size)

	sendTimes = make([]int64, n+1)
	latencies = make([]float64, n)
	atomic.StoreInt64(&latencyIdx, 0)
	atomic.StoreInt64(&totalSent, 0)
	atomic.StoreInt64(&totalRecv, 0)
	atomic.StoreInt64(&dialErrors, 0)
	atomic.StoreInt64(&pushErrors, 0)
	connDurations = nil

	wg.Add(n)

	chSeq := make(chan int32, n)

	// 建立连接，记录连接耗时
	for range c {
		dialStart := time.Now()
		conn, err := proxy.Dial()
		if err != nil {
			log.Errorf("gate connect failed: %v", err)
			atomic.AddInt64(&dialErrors, 1)
			return
		}

		connMu.Lock()
		connDurations = append(connDurations, float64(time.Since(dialStart))/float64(time.Millisecond))
		connMu.Unlock()

		go func(conn *client.Conn) {
			defer func() {
				_ = conn.Close()
			}()

			for {
				seq, ok := <-chSeq
				if !ok {
					return
				}

				sendTimes[seq] = time.Now().UnixNano()

				if err := conn.Push(&cluster.Message{
					Route: 1,
					Seq:   seq,
					Data:  &greetReq{Message: message},
				}); err != nil {
					log.Errorf("push message failed: %v", err)
					atomic.AddInt64(&pushErrors, 1)
					wg.Done()
					return
				}

				atomic.AddInt64(&totalSent, 1)
			}
		}(conn)
	}

	startTime = time.Now().UnixNano()

	for i := 1; i <= n; i++ {
		chSeq <- int32(i)
	}

	wg.Wait()

	close(chSeq)

	duration := float64(time.Now().UnixNano()-startTime) / float64(time.Second)
	recv := atomic.LoadInt64(&totalRecv)
	dialErr := atomic.LoadInt64(&dialErrors)
	pushErr := atomic.LoadInt64(&pushErrors)

	// 延迟统计
	latencies = latencies[:recv]
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
	if sent := atomic.LoadInt64(&totalSent); sent > 0 {
		successRate = float64(recv) / float64(sent) * 100
	}

	// 输出结果
	fmt.Printf("\n")
	fmt.Printf("%s\n", sectionLineWith("WS BENCHMARK", '='))
	fmt.Printf("  Concurrency: %-5d ｜ Requests: %-9d ｜ Size: %s\n",
		c, n, convBytes(size))
	fmt.Printf("================================================================\n")
	fmt.Printf("  Protocol:            %s\n", proxy.Client().Protocol())
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
	fmt.Printf("  Sent:                %s\n", formatNum(atomic.LoadInt64(&totalSent)))
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
