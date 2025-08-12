package log

type Option func(o *options)

type options struct {
	outLevel      Level      // 输出级别
	outFormat     Format     // 输出格式
	outTerminals  []Terminal // 输出终端
	outStackLevel Level      // 输出栈的日志级别
	outStackDepth int        // 输出栈的深度
	filePath      string     // 文件路径
	fileMaxAge    int64      // 文件最大留存时间
	fileMaxSize   int64      // 单个文件最大尺寸
	fileRotate    FileRotate // 文件反转规则
	timeZone      string     // 时间时区，默认为Local
	timeFormat    string     // 时间格式，标准库时间格式，默认2006/01/02 15:04:05.000000
	compress      bool       // 是否对轮换的日志文件进行压缩
}

// WithOutLevel 设置日志的输出级别
func WithOutLevel(level Level) Option {
	return func(o *options) { o.outLevel = level }
}

// WithOutFormat 设置日志的输出格式
func WithOutFormat(format Format) Option {
	return func(o *options) { o.outFormat = format }
}

// WithOutTerminal 设置日志的输出终端
func WithOutTerminal(terminals ...Terminal) Option {
	return func(o *options) { o.outTerminals = terminals }
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

// WithTimeZone 设置日志文件打印时间的时区
func WithTimeZone(timeZone string) Option {
	return func(o *options) { o.timeZone = timeZone }
}

// WithTimeFormat 设置日志输出时间格式
func WithTimeFormat(timeFormat string) Option {
	return func(o *options) { o.timeFormat = timeFormat }
}

// WithCompress 设置是否对轮换日志文件进行压缩
func WithCompress(compress bool) Option {
	return func(o *options) { o.compress = compress }
}
