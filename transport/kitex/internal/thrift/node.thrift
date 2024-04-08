include "message.thrift"

struct TriggerRequest {
    1: i8 Event // 事件
    2: string GID // 网关ID
    3: i64 CID // 连接ID
    4: i64 UID // 用户ID
}

struct TriggerResponse {

}

struct DeliverRequest {
    1: string GID // 网关ID
    2: string NID // 节点ID
    3: i64 CID // 连接ID
    4: i64 UID // 用户ID
    5: message.Message Message // 消息
}

struct DeliverResponse {

}

service Node {
    // 触发事件
    TriggerResponse Trigger(1: TriggerRequest req)
    // 投递消息
    DeliverResponse Deliver(1: DeliverRequest req)
}