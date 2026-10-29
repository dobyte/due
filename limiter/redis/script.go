package redis

// allowScript atomically refills and takes tokens from a token bucket.
//
// KEYS[1] is the bucket key. ARGV[1] is the fallback current time in milliseconds, ARGV[2] the
// refill rate in tokens per second, ARGV[3] the bucket capacity, ARGV[4] the cost and ARGV[5] the
// key expiration in milliseconds.
//
// The script prefers the redis server time so that clock skew between application nodes does not
// affect the refill. The fallback is used only when the server time is unavailable. It returns
// {allowed, remaining, retryAfterMs, resetAfterMs}.
const allowScript = `
if redis.replicate_commands then
  redis.replicate_commands()
end

local now
local t = redis.pcall('TIME')
if type(t) == 'table' and not t.err then
  now = tonumber(t[1]) * 1000 + math.floor(tonumber(t[2]) / 1000)
else
  now = tonumber(ARGV[1])
end

local rate  = tonumber(ARGV[2])
local burst = tonumber(ARGV[3])
local cost  = tonumber(ARGV[4])

local vals = redis.call('HMGET', KEYS[1], 'tokens', 'ts')
local tokens = tonumber(vals[1])
local ts = tonumber(vals[2])

if tokens == nil then
  tokens = burst
  ts = now
end

if now < ts then
  now = ts
end

local elapsed = now - ts
tokens = math.min(burst, tokens + elapsed * rate / 1000)

local allowed = 0
if tokens >= cost then
  tokens = tokens - cost
  allowed = 1
end

local retry = 0
if allowed == 0 then
  retry = math.ceil((cost - tokens) * 1000 / rate)
end

local reset = 0
if tokens < burst then
  reset = math.ceil((burst - tokens) * 1000 / rate)
end

redis.call('HSET', KEYS[1], 'tokens', tostring(tokens), 'ts', tostring(now))
redis.call('PEXPIRE', KEYS[1], ARGV[5])

return {allowed, math.floor(tokens), retry, reset}
`
