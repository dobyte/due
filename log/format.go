/**
 * @Author: fuxiao
 * @Email: 576101059@qq.com
 * @Date: 2022/8/30 5:08 下午
 * @Desc: TODO
 */

package log

type Format int

const (
	TextFormat Format = iota // 文本格式
	JsonFormat               // JSON格式
)
