/**
 * @Author: fuxiao
 * @Email: 576101059@qq.com
 * @Date: 2022/8/29 7:27 下午
 * @Desc: TODO
 */

package logrus

import "github.com/dobyte/due/log"

type options struct {
	level log.Level
}

type Option func(o *options)


func WithLevel(level log.Level) Option {
	return func(o *options) { o.level = level }
}
