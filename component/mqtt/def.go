package mqtt

import (
	mqtt "github.com/mochi-mqtt/server/v2"
	"github.com/mochi-mqtt/server/v2/packets"
)

type (
	Client           = mqtt.Client
	ClientState      = mqtt.ClientState
	ClientProperties = mqtt.ClientProperties
	ClientConnection = mqtt.ClientConnection
	Packet           = packets.Packet
	Hook             = mqtt.Hook
	HookBase         = mqtt.HookBase
	Will             = mqtt.Will
)

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
