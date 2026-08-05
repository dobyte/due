package redis

// 释放锁
const releaseScript = `
	local val = redis.call('GET', KEYS[1])

	if not val then
		return 1
	end

	if val ~= ARGV[1] then
		return 0
	end

	redis.call('DEL', KEYS[1])

	return 1
`

// 续租锁
const renewalScript = `
	local val = redis.call('GET', KEYS[1])

	if not val then
		return 0
	end

	if val ~= ARGV[1] then
		return 0
	end

	redis.call('PEXPIRE', KEYS[1], ARGV[2])

	return 1
`
