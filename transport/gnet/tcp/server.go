package tcp

import (
	"context"
	"fmt"
	"github.com/golang/protobuf/proto"
	"github.com/panjf2000/gnet/v2"
	"github.com/symsimmy/due/errors"
	"github.com/symsimmy/due/internal/endpoint"
	"github.com/symsimmy/due/internal/prom"
	"github.com/symsimmy/due/internal/util"
	"github.com/symsimmy/due/log"
	"github.com/symsimmy/due/transport"
	"time"
)

const scheme = "tcp"

type Server struct {
	gnet.BuiltinEventEngine
	eng            gnet.Engine
	network        string
	addr           string
	multicore      bool
	endpoint       *endpoint.Endpoint
	receiveHandler transport.ReceiveHandler // 接收消息hook函数
}

func NewServer(addr string) *Server {
	ss := &Server{
		network:   scheme,
		addr:      addr,
		multicore: true,
	}

	return ss
}

func (s *Server) OnBoot(eng gnet.Engine) (action gnet.Action) {
	log.Infof("running server on %s with multi-core=%t",
		fmt.Sprintf("%s://%s", s.network, s.addr), s.multicore)
	s.eng = eng
	return
}

func (s *Server) OnOpen(c gnet.Conn) (out []byte, action gnet.Action) {
	c.SetContext(new(SimpleCodec))
	return
}

func (s *Server) OnClose(c gnet.Conn, err error) (action gnet.Action) {
	if err != nil {
		//logging.Infof("error occurred on connection=%s, %v\n", c.RemoteAddr().String(), err)
	}
	return
}

func (s *Server) OnTraffic(c gnet.Conn) (action gnet.Action) {
	codec := c.Context().(*SimpleCodec)
	for {
		data, err := codec.Decode(c)
		if errors.Is(err, ErrIncompletePacket) {
			break
		}
		if err != nil {
			log.Warnf("invalid packet: %v", err)
			return gnet.Close
		}
		// 收到 ping 包
		if len(data.Data) <= 0 {
			break
		}
		// 返回消息
		start := time.Now()
		v, err := s.receiveHandler(data.Route, data.Data)
		prom.ServerRpcHandleDurationSummary.WithLabelValues(s.addr, util.ToString(data.Route)).Observe(float64(time.Since(start).Milliseconds()))

		if err != nil {
			log.Debugf("server receive messageId:[%+v],handler[route=%+v] failed.err:%+v", data.Route, data.MessageId, err)
			prom.ServerReceiveHandleError.WithLabelValues(err.Error())
			continue
		}

		if v != nil {
			reply, ok := v.(proto.Message)
			if ok {
				replyBytes, _ := proto.Marshal(reply)
				packet, _ := codec.Encode(data.Route, data.MessageId, replyBytes)
				start = time.Now()
				_, err = c.Write(packet)
				prom.ServerRpcWriteDurationSummary.WithLabelValues(s.addr, util.ToString(data.Route)).Observe(float64(time.Since(start).Milliseconds()))
				if err != nil {
					prom.ServerInternalWriteErrorCounter.WithLabelValues(err.Error()).Inc()
				}
			}
		}
	}

	return
}

// OnReceive 监听接收到消息
func (s *Server) OnReceive(handler transport.ReceiveHandler) {
	s.receiveHandler = handler
}

func (s *Server) Start() {
	go func() {
		err := gnet.Run(s,
			s.network+"://"+s.addr,
			gnet.WithMulticore(s.multicore),
			gnet.WithReuseAddr(true),
		)
		if err != nil {
			log.Errorf("server %+v exits with error: %v", s.addr, err)
		} else {
			log.Infof("server %+v exit", s.addr)
		}
	}()
}

func (s *Server) Stop() {
	err := s.eng.Stop(context.Background())
	if err != nil {
		return
	}
}
