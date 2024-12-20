package log

const (
	defaultFilePath    = "./log/due.log"
	defaultFileMaxAge  = 24 * 3600
	defaultFileMaxSize = 100 * MB
	defaultFileRotate  = FileRotateNone
	defaultTimezone    = "Local"
)

type Option func(o *options)

type options struct {
	filePath    string     // 文件路径
	fileMaxAge  int64      // 文件最大留存时间
	fileMaxSize int64      // 单个文件最大尺寸
	fileRotate  FileRotate // 文件反转规则
	timezone    string     // 时间时区，默认为Local
	compress    bool       // 是否对轮换的日志文件进行压缩
}

func defaultOptions() *options {
	return &options{
		filePath:    defaultFilePath,
		fileMaxAge:  defaultFileMaxAge,
		fileMaxSize: defaultFileMaxSize,
		fileRotate:  defaultFileRotate,
		timezone:    defaultTimezone,
	}
}

// WithFilePath 设置文件路径
func WithFilePath(filePath string) Option {
	return func(o *options) { o.filePath = filePath }
}

// WithFileMaxAge 设置文件最大留存时间
func WithFileMaxAge(fileMaxAge int64) Option {
	return func(o *options) { o.fileMaxAge = fileMaxAge }
}

// WithFileMaxSize 设置单个文件最大尺寸
func WithFileMaxSize(fileMaxSize int64) Option {
	return func(o *options) { o.fileMaxSize = fileMaxSize }
}

// WithFileRotate 设置文件反转规则
func WithFileRotate(fileRotate FileRotate) Option {
	return func(o *options) { o.fileRotate = fileRotate }
}

// WithTimezone 设置日志文件打印时间的时区
func WithTimezone(timezone string) Option {
	return func(o *options) { o.timezone = timezone }
}

// WithCompress 设置是否对轮换日志文件进行压缩
func WithCompress(compress bool) Option {
	return func(o *options) { o.compress = compress }
}
