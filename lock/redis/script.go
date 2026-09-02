package redis

// 释放锁脚本
// 仅当锁存在且版本标识与 ARGV[1] 匹配时删除锁；
// 锁不存在或所有权已变更(版本标识不匹配)时返回0，使释放方能够感知锁已丢失，语义与 memcache 版一致
const releaseScript = `
	local val = redis.call('GET', KEYS[1])

	if not val then
		return 0
	end

	if val ~= ARGV[1] then
		return 0
	end

	redis.call('DEL', KEYS[1])

	return 1
`

// 续租锁脚本
// 仅当锁存在且版本标识与 ARGV[1] 匹配时，将锁的过期时间刷新为 ARGV[2](毫秒)；
// 锁不存在或所有权已变更(版本标识不匹配)时返回0，表示锁已丢失
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
