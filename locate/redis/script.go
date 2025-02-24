package redis

// 解绑网关脚本
const unbindGateScript = `
	local val = redis.call('GET', KEYS[1])

	if val == '' or val ~= ARGV[1] then
		return {'NO'}
	end

	redis.call('DEL', KEYS[1])

	return {'OK'}
`

// 解绑节点脚本
const unbindNodeScript = `
	local val = redis.call('HGET', KEYS[1], ARGV[1])

	if val == '' or val ~= ARGV[2] then
		return {'NO'}
	end

	redis.call('HDEL', KEYS[1], ARGV[1])

	return {'OK'}
`
