package tencent

import (
	"fmt"
	"sync"

	"github.com/dobyte/due/v2/encoding/json"
	"github.com/dobyte/due/v2/log"
	cls "github.com/tencentcloud/tencentcloud-cls-sdk-go"
)

const (
	fieldKeyLevel = "level"
	fieldKeyTime  = "time"
	fieldKeyFile  = "file"
	fieldKeyMsg   = "msg"
	fieldKeyStack = "stack"
)

// Name is the syncer name.
const Name = "tencent"

// Syncer is a Tencent Cloud CLS log syncer.
type Syncer struct {
	opts     *options
	producer *cls.AsyncProducerClient
	rawPool  sync.Pool
}

// stackFrame is a stack frame.
type stackFrame struct {
	Func string `json:"func"`
	File string `json:"file"`
}

// NewSyncer returns a new Tencent Cloud CLS log syncer. The optional opts configure the syncer.
func NewSyncer(opts ...Option) *Syncer {
	o := defaultOptions()
	for _, opt := range opts {
		opt(o)
	}

	config := cls.GetDefaultAsyncProducerClientConfig()
	config.Endpoint = o.endpoint
	config.AccessKeyID = o.accessKeyID
	config.AccessKeySecret = o.accessKeySecret

	producer, err := cls.NewAsyncProducerClient(config)
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

// Write writes the given entity to Tencent Cloud CLS. It returns any error encountered while writing.
func (s *Syncer) Write(entity *log.Entity) error {
	return s.producer.SendLog(s.opts.topicID, s.makeLog(entity), nil)
}

// Close closes the Tencent Cloud CLS producer.
func (s *Syncer) Close() error {
	return s.producer.Close(60000)
}

// makeLog converts an entity into a CLS log.
func (s *Syncer) makeLog(entity *log.Entity) *cls.Log {
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

	return cls.NewCLSLog(entity.Now.Unix(), raw)
}
