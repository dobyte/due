package tcp

import (
	"context"
	"fmt"
	"github.com/golang/protobuf/proto"
	"github.com/panjf2000/gnet/v2"
	"github.com/symsimmy/due/errors"
	"github.com/symsimmy/due/common/endpoint"
	"github.com/symsimmy/due/log"
	"github.com/symsimmy/due/transport"
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
		v, err := s.receiveHandler(data.Route, data.Data)

		if err != nil {
			log.Debugf("server receive messageId:[%+v],handler[route=%+v] failed.err:%+v", data.Route, data.MessageId, err)

			continue
		}

		if v != nil {
			reply, ok := v.(proto.Message)
			if ok {
				replyBytes, _ := proto.Marshal(reply)
				packet, _ := codec.Encode(data.Route, data.MessageId, replyBytes)
				_, err = c.Write(packet)
				if err != nil {

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
