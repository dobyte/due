package redis

// releaseScript releases a lock. It deletes the lock only when the lock exists and its version
// identifier matches ARGV[1]. When the lock is missing or its ownership has changed (the version
// identifier does not match), it returns 0 so that the releaser can detect the lost lock; this
// matches the semantics of the memcache implementation.
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

// renewalScript renews a lock. It refreshes the lock expiration to ARGV[2] milliseconds only when
// the lock exists and its version identifier matches ARGV[1]. When the lock is missing or its
// ownership has changed (the version identifier does not match), it returns 0, meaning the lock has
// been lost.
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
