package drpc

import (
	"bytes"
	"encoding/binary"
)

const (
	defaultSizeBytes   = 4
	defaultHeaderBytes = 1
)

const (
	cmdBind       int8 = iota // 绑定用户
	cmdUnbind                 // 解绑用户
	cmdGetIP                  // 获取IP地址
	cmdIsOnline               // 检测是否在线
	cmdStat                   // 统计在线人数
	cmdDisconnect             // 断开连接
	cmdPush                   // 推送消息
	cmdMulticast              // 推送组播消息
	cmdBroadcast              // 推送广播消息

	heartbeatPacket = 1 << 7 // 心跳包
)

// 0 1 2 3 4 5 6 7 0 1 2 3 4 5 6 7 0 1 2 3 4 5 6 7 0 1 2 3 4 5 6 7 0 1 2 3 4 5 6 7 0 1 2 3 4 5 6 7 0 1 2 3 4 5 6 7 0 1 2 3 4 5 6 7 0 1 2 3 4 5 6 7
// +---------------------------------------------------------------+-+-------------+-------------------------------+-------------------------------+
// |                              size                             |h|   extcode   |             route             |              seq              |
// +---------------------------------------------------------------+-+-------------+-------------------------------+-------------------------------+
// |                                                                message data ...                                                               |
// +-----------------------------------------------------------------------------------------------------------------------------------------------+

// PackBindCMD 打包绑定用户命令
func PackBindCMD(cid, uid int64) ([]byte, error) {
	buf := &bytes.Buffer{}

	size := defaultHeaderBytes + 8 + 8

	buf.Grow(defaultSizeBytes + size)

	err := binary.Write(buf, binary.BigEndian, int32(size))
	if err != nil {
		return nil, err
	}

	err = binary.Write(buf, binary.BigEndian, cmdBind)
	if err != nil {
		return nil, err
	}

	err = binary.Write(buf, binary.BigEndian, cid)
	if err != nil {
		return nil, err
	}

	err = binary.Write(buf, binary.BigEndian, uid)
	if err != nil {
		return nil, err
	}

	return buf.Bytes(), nil
}

func UnpackBindCMD() {

}
