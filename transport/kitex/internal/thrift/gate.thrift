include "message.thrift"

struct BindRequest {
    1: i64 CID
    2: i64 UID
}

struct BindResponse {

}

struct UnbindRequest {
    1: i64 UID
}

struct UnbindResponse {

}

struct GetIPRequest {
    1: i8 Kind // 类型 1：CID 2：UID
    2: i64 Target // 目标
}

struct GetIPResponse {
    1: string IP // IP地址
}

struct StatRequest {
    1: i8 Kind // 类型 1：CID 2：UID
}

struct StatResponse {
    1: i64 Total // 在线总数
}

struct DisconnectRequest {
    1: i8 Kind // 类型 1：CID 2：UID
    2: i64 Target // 目标
    3: bool IsForce // 是否强制关闭
}

struct DisconnectResponse {

}

struct PushRequest {
    1: i8 Kind // 类型 1：CID 2：UID
    2: i64 Target // 目标
    3: message.Message Message // 消息
}

struct PushResponse {

}

struct MulticastRequest {
    1: i8 Kind // 类型 1：CID 2：UID
    2: list<i64> Targets // 目标
    3: message.Message Message // 消息
}

struct MulticastResponse {
    1: i64 Total // 组播数量
}

struct BroadcastRequest {
    1: i8 Kind // 类型 1：CID 2：UID
    2: message.Message Message // 消息
}

struct BroadcastResponse {
    1: i64 Total // 广播数量
}

service Gate {
    // 绑定用户与连接
    BindResponse Bind(1: BindRequest req)
    // 解绑用户与连接
    UnbindResponse Unbind(1: UnbindRequest req)
    // 获取客户端IP
    GetIPResponse GetIP(1: GetIPRequest req)
    // 统计会话总数
    StatResponse Stat(1: StatRequest req)
    // 断开连接
    DisconnectResponse Disconnect(1: DisconnectRequest req)
    // 推送消息
    PushResponse Push(1: PushRequest req)
    // 推送组播消息
    MulticastResponse Multicast(1: MulticastRequest req)
    // 推送广播消息
    BroadcastResponse Broadcast(1: BroadcastRequest req)
}