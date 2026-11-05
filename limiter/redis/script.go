package redis

// tokenScale is the number of fixed-point units in one token. Tokens are stored as integer
// nano-tokens so that the Lua script avoids floating-point token storage and formatting.
const tokenScale int64 = 1_000_000_000

// maxExactLuaInteger is the largest integer that Lua numbers (IEEE-754 doubles) represent exactly.
const maxExactLuaInteger int64 = 1<<53 - 1

// allowScript atomically refills and takes tokens from a token bucket.
//
// KEYS[1] is the bucket key. ARGV[1] is the refill rate in nano-tokens per millisecond, ARGV[2]
// the bucket capacity in nano-tokens, ARGV[3] the cost in nano-tokens and ARGV[4] the key
// expiration in milliseconds.
//
// State is stored as "tokens|ts": tokens is an integer number of nano-tokens and ts is the last
// refill time in milliseconds. Legacy hashes holding float "tokens" and "ts" fields are migrated
// once. The script uses the Redis server time so that clock skew between application nodes does
// not affect the refill. Script effects replication must be available: Redis 3.2+ can enable it
// with redis.replicate_commands, and Redis 7+ enables it by default.
//
// It returns {allowed, tokensNano, retryAfterMs, resetAfterMs}.
const allowScript = `
if redis.replicate_commands then
  redis.replicate_commands()
end

local t = redis.call('TIME')
local now = tonumber(t[1]) * 1000 + math.floor(tonumber(t[2]) / 1000)

local rate  = tonumber(ARGV[1])
local burst = tonumber(ARGV[2])
local cost  = tonumber(ARGV[3])

local raw = redis.pcall('GET', KEYS[1])
local tokens
local ts

if type(raw) == 'string' then
  local sep = string.find(raw, '|', 1, true)
  if sep then
    tokens = tonumber(string.sub(raw, 1, sep - 1))
    ts = tonumber(string.sub(raw, sep + 1))
  end
elseif type(raw) == 'table' and raw.err then
  -- Legacy hash representation; convert float tokens to nano-tokens once.
  local vals = redis.call('HMGET', KEYS[1], 'tokens', 'ts')
  local legacyTokens = tonumber(vals[1])
  local legacyTs = tonumber(vals[2])
  if legacyTokens ~= nil and legacyTs ~= nil then
    tokens = math.floor(legacyTokens * 1000000000)
    ts = legacyTs
  end
  redis.call('DEL', KEYS[1])
end

if tokens == nil or ts == nil then
  tokens = burst
  ts = now
end

if tokens < 0 then
  tokens = 0
elseif tokens > burst then
  tokens = burst
end

if now < ts then
  now = ts
end

if tokens < burst then
  local needed = burst - tokens
  local maxElapsed = math.ceil(needed / rate)
  local elapsed = now - ts
  if elapsed > maxElapsed then
    elapsed = maxElapsed
  end
  tokens = tokens + elapsed * rate
  if tokens > burst then
    tokens = burst
  end
end

ts = now

local allowed = 0
if tokens >= cost then
  tokens = tokens - cost
  allowed = 1
end

local retry = 0
if allowed == 0 then
  retry = math.ceil((cost - tokens) / rate)
end

local reset = 0
if tokens < burst then
  reset = math.ceil((burst - tokens) / rate)
end

redis.call('SET', KEYS[1], string.format('%.0f', tokens) .. '|' .. string.format('%.0f', now), 'PX', ARGV[4])

return {allowed, tokens, retry, reset}
`
