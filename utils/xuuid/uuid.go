// Package xuuid 提供 UUID 生成工具。
package xuuid

import "uuid"

// UUID 生成一个新的 UUID（版本7）字符串
// 版本7 UUID 基于时间戳（毫秒级）+随机数生成，在时间上近似单调递增，
// 比完全随机的版本4更适合作为数据库主键或分布式ID，可减少索引碎片。
// @return @1 string 生成的 UUID 字符串
func UUID() string {
	return uuid.NewV7().String()
}
