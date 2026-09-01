package redis

import (
	"encoding/json"
	"slices"
	"strings"

	"github.com/dobyte/due/v2/locate"
)

// 序列化定位事件
// 将定位事件编码为JSON字符串，用于通过Redis发布订阅进行广播
// @param event *locate.Event 待序列化的定位事件
// @return @1 string 序列化后的JSON字符串
// @return @2 error 序列化失败时返回的错误
func marshal(event *locate.Event) (string, error) {
	buf, err := json.Marshal(event)
	if err != nil {
		return "", err
	}

	return string(buf), nil
}

// 反序列化定位事件
// 将JSON字符串解码为定位事件
// @param data []byte 待反序列化的JSON数据
// @return @1 *locate.Event 反序列化得到的定位事件
// @return @2 error 反序列化失败时返回的错误
func unmarshal(data []byte) (*locate.Event, error) {
	evt := &locate.Event{}

	if err := json.Unmarshal(data, evt); err != nil {
		return nil, err
	}

	return evt, nil
}

// 生成唯一键
// 将多个实例类型排序后拼接为唯一键，用于复用相同实例类型组合的监听管理器
// @param kinds ...string 实例类型列表
// @return @1 string 排序并拼接后的唯一键
func toUniqueKey(kinds ...string) string {
	keys := make([]string, len(kinds))
	copy(keys, kinds)
	slices.Sort(keys)

	return strings.Join(keys, "&")
}
