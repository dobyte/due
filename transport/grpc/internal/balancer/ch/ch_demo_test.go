package ch

import (
	"context"
	"strconv"
	"testing"

	"google.golang.org/grpc/balancer"
)

// TestConsistentHashDemo 使用 mock 集群数据本地演示一致性哈希路由效果
func TestConsistentHashDemo(t *testing.T) {
	// mock 集群：模拟服务发现返回的 5 个实例地址
	cluster := []*hashSubConn{
		{sc: &mockSubConn{}, key: "192.168.1.10:8011"},
		{sc: &mockSubConn{}, key: "192.168.1.11:8011"},
		{sc: &mockSubConn{}, key: "192.168.1.12:8011"},
		{sc: &mockSubConn{}, key: "192.168.1.13:8011"},
		{sc: &mockSubConn{}, key: "192.168.1.14:8011"},
	}

	// mock 请求：1000 个用户 ID 访问同一 RPC 方法
	const userCount = 1000
	users := make([]string, 0, userCount)
	for i := 0; i < userCount; i++ {
		users = append(users, "user-"+strconv.Itoa(i))
	}

	t.Run("按用户ID哈希（粘性路由）", func(t *testing.T) {
		p := &Picker{ring: newConsistentRing(cluster)}

		counts := make(map[balancer.SubConn]int)
		sticky := make(map[string]balancer.SubConn)
		for _, u := range users {
			ctx := WithHashKey(context.Background(), u)
			res, err := p.Pick(balancer.PickInfo{Ctx: ctx, FullMethodName: "/user.Service/GetUserInfo"})
			if err != nil {
				t.Fatal(err)
			}
			counts[res.SubConn]++
			sticky[u] = res.SubConn
		}

		// 同一用户再次访问必须命中同一节点
		for i := 0; i < 200; i++ {
			u := users[i]
			ctx := WithHashKey(context.Background(), u)
			res, err := p.Pick(balancer.PickInfo{Ctx: ctx, FullMethodName: "/user.Service/GetUserInfo"})
			if err != nil {
				t.Fatal(err)
			}
			if res.SubConn != sticky[u] {
				t.Fatalf("user %s 未命中同一节点", u)
			}
		}
		t.Log("粘性校验通过：同一用户 200 次重复访问均命中同一节点")

		printDistribution(t, "按用户ID哈希的节点分布", cluster, counts)
	})

	t.Run("按方法名哈希", func(t *testing.T) {
		p := &Picker{ring: newConsistentRing(cluster)}

		counts := make(map[balancer.SubConn]int)
		for i := 0; i < userCount; i++ {
			res, err := p.Pick(balancer.PickInfo{FullMethodName: "/user.Service/GetUserInfo"})
			if err != nil {
				t.Fatal(err)
			}
			counts[res.SubConn]++
		}

		if len(counts) != 1 {
			t.Errorf("同一方法应固定路由到同一节点，实际命中 %d 个节点", len(counts))
		}

		printDistribution(t, "按方法名哈希的节点分布", cluster, counts)
	})

	t.Run("节点故障迁移", func(t *testing.T) {
		p1 := &Picker{ring: newConsistentRing(cluster)}

		initial := make(map[string]balancer.SubConn)
		for _, u := range users {
			res, err := p1.Pick(balancer.PickInfo{FullMethodName: u})
			if err != nil {
				t.Fatal(err)
			}
			initial[u] = res.SubConn
		}

		// 模拟 192.168.1.14:8011 宕机下线，环仅剩 4 个节点
		p2 := &Picker{ring: newConsistentRing(cluster[:4])}
		moved := 0
		for _, u := range users {
			res, err := p2.Pick(balancer.PickInfo{FullMethodName: u})
			if err != nil {
				t.Fatal(err)
			}
			if res.SubConn != initial[u] {
				moved++
			}
		}

		pct := float64(moved) / float64(userCount) * 100
		t.Logf("移除 1/5 节点(192.168.1.14)后，迁移键 %d/%d = %.1f%%（理论约 20%%）", moved, userCount, pct)

		// 5 节点移除 1 个，迁移率应在 20% 附近
		if moved < 120 || moved > 280 {
			t.Errorf("迁移率异常: %d/%d (%.1f%%)", moved, userCount, pct)
		}
	})
}

// printDistribution 打印各节点的请求分布
func printDistribution(t *testing.T, title string, cluster []*hashSubConn, counts map[balancer.SubConn]int) {
	t.Helper()
	total := 0
	for _, c := range counts {
		total += c
	}
	if total == 0 {
		return
	}

	t.Logf("-- %s --", title)
	for _, node := range cluster {
		n := counts[node.sc]
		pct := float64(n) / float64(total) * 100
		t.Logf("  %-18s %5d 请求 %6.1f%%", node.key, n, pct)
	}
}
