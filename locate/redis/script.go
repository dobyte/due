package redis

// unbindGateScript is the Lua script that unbinds a gate.
const unbindGateScript = `
	local val = redis.call('GET', KEYS[1])

	if val ~= ARGV[1] then
		return {'NO'}
	end

	redis.call('DEL', KEYS[1])

	return {'OK'}
`

// unbindNodeScript is the Lua script that unbinds a node.
const unbindNodeScript = `
	local val = redis.call('HGET', KEYS[1], ARGV[1])

	if val ~= ARGV[2] then
		return {'NO'}
	end

	redis.call('HDEL', KEYS[1], ARGV[1])

	return {'OK'}
`
