package mqtt

import (
	mqtt "github.com/mochi-mqtt/server/v2"
	"github.com/mochi-mqtt/server/v2/packets"
)

// 对外暴露的mochi-mqtt类型别名
type (
	Client           = mqtt.Client           // 客户端
	ClientState      = mqtt.ClientState      // 客户端状态
	ClientProperties = mqtt.ClientProperties // 客户端属性
	ClientConnection = mqtt.ClientConnection // 客户端连接信息
	Packet           = packets.Packet        // MQTT数据包
	Hook             = mqtt.Hook             // Hook接口
	HookBase         = mqtt.HookBase         // Hook基类
	Will             = mqtt.Will             // 遗嘱消息
)

// 对外暴露的mochi-mqtt Hook事件及存储能力常量
const (
	SetOptions             = mqtt.SetOptions
	OnSysInfoTick          = mqtt.OnSysInfoTick
	OnStarted              = mqtt.OnStarted
	OnStopped              = mqtt.OnStopped
	OnConnectAuthenticate  = mqtt.OnConnectAuthenticate
	OnACLCheck             = mqtt.OnACLCheck
	OnConnect              = mqtt.OnConnect
	OnSessionEstablish     = mqtt.OnSessionEstablish
	OnSessionEstablished   = mqtt.OnSessionEstablished
	OnDisconnect           = mqtt.OnDisconnect
	OnAuthPacket           = mqtt.OnAuthPacket
	OnPacketRead           = mqtt.OnPacketRead
	OnPacketEncode         = mqtt.OnPacketEncode
	OnPacketSent           = mqtt.OnPacketSent
	OnPacketProcessed      = mqtt.OnPacketProcessed
	OnSubscribe            = mqtt.OnSubscribe
	OnSubscribed           = mqtt.OnSubscribed
	OnSelectSubscribers    = mqtt.OnSelectSubscribers
	OnUnsubscribe          = mqtt.OnUnsubscribe
	OnUnsubscribed         = mqtt.OnUnsubscribed
	OnPublish              = mqtt.OnPublish
	OnPublished            = mqtt.OnPublished
	OnPublishDropped       = mqtt.OnPublishDropped
	OnRetainMessage        = mqtt.OnRetainMessage
	OnRetainPublished      = mqtt.OnRetainPublished
	OnQosPublish           = mqtt.OnQosPublish
	OnQosComplete          = mqtt.OnQosComplete
	OnQosDropped           = mqtt.OnQosDropped
	OnPacketIDExhausted    = mqtt.OnPacketIDExhausted
	OnWill                 = mqtt.OnWill
	OnWillSent             = mqtt.OnWillSent
	OnClientExpired        = mqtt.OnClientExpired
	OnRetainedExpired      = mqtt.OnRetainedExpired
	StoredClients          = mqtt.StoredClients
	StoredSubscriptions    = mqtt.StoredSubscriptions
	StoredInflightMessages = mqtt.StoredInflightMessages
	StoredRetainedMessages = mqtt.StoredRetainedMessages
	StoredSysInfo          = mqtt.StoredSysInfo
)
