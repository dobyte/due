package aliyun

import (
	"fmt"
	"sync"

	sls "github.com/aliyun/aliyun-log-go-sdk"
	"github.com/aliyun/aliyun-log-go-sdk/producer"
	"github.com/dobyte/due/v2/encoding/json"
	"github.com/dobyte/due/v2/log"
)

const (
	fieldKeyLevel = "level"
	fieldKeyTime  = "time"
	fieldKeyFile  = "file"
	fieldKeyMsg   = "msg"
	fieldKeyStack = "stack"
)

// Name is the syncer name.
const Name = "aliyun"

// Syncer is an Aliyun SLS log syncer.
type Syncer struct {
	opts     *options
	producer *producer.Producer
	rawPool  sync.Pool
}

// stackFrame is a stack frame.
type stackFrame struct {
	Func string `json:"func"`
	File string `json:"file"`
}

// NewSyncer returns a new Aliyun SLS log syncer. The optional opts configure the syncer.
func NewSyncer(opts ...Option) *Syncer {
	o := defaultOptions()
	for _, opt := range opts {
		opt(o)
	}

	config := producer.GetDefaultProducerConfig()
	config.Endpoint = o.endpoint
	config.CredentialsProvider = sls.NewStaticCredentialsProvider(o.accessKeyID, o.accessKeySecret, "")
	config.AllowLogLevel = "error"

	producer, err := producer.NewProducer(config)
	if err != nil {
		return nil
	} else {
		producer.Start()
	}

	s := &Syncer{}
	s.opts = o
	s.producer = producer
	s.rawPool = sync.Pool{New: func() any { return make(map[string]string, 5) }}

	return s
}

// Name returns the syncer name.
func (s *Syncer) Name() string {
	return Name
}

// Write writes the given entity to Aliyun SLS. It returns any error encountered while writing.
func (s *Syncer) Write(entity *log.Entity) error {
	return s.producer.SendLog(s.opts.project, s.opts.logstore, s.opts.topic, s.opts.source, s.makeLog(entity))
}

// Close closes the Aliyun SLS producer.
func (s *Syncer) Close() error {
	return s.producer.Close(5000)
}

// makeLog converts an entity into an SLS log.
func (s *Syncer) makeLog(entity *log.Entity) *sls.Log {
	raw := s.rawPool.Get().(map[string]string)
	defer func() {
		clear(raw)
		s.rawPool.Put(raw)
	}()

	raw[fieldKeyLevel] = string(entity.Level[:4])
	raw[fieldKeyTime] = entity.Time
	raw[fieldKeyFile] = entity.Caller
	raw[fieldKeyMsg] = entity.Message

	if len(entity.Frames) > 0 {
		frames := make([]stackFrame, 0, len(entity.Frames))
		for _, f := range entity.Frames {
			frames = append(frames, stackFrame{
				Func: f.Function,
				File: fmt.Sprintf("%s:%d", f.File, f.Line),
			})
		}

		data, _ := json.Marshal(frames)
		raw[fieldKeyStack] = string(data)
	}

	return producer.GenerateLog(uint32(entity.Now.Unix()), raw)
}
