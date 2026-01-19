package ratelimiter

const tokenBucketLua = `
local key = KEYS[1]
local capacity = tonumber(ARGV[1])
local rate = tonumber(ARGV[2])
local ttl = tonumber(ARGV[3])

local now_data = redis.call("TIME")
local now = tonumber(now_data[1]) + (tonumber(now_data[2]) / 1000000)

local data = redis.call("HMGET", key, "tokens", "ts")
local tokens = tonumber(data[1])
local ts = tonumber(data[2])

if tokens == nil then
  tokens = capacity
  ts = now
else
  local delta = math.max(0, now - ts)
  tokens = math.min(capacity, tokens + (delta * rate))
  ts = now
end

local allowed = 0
if tokens >= 1 then
  allowed = 1
  tokens = tokens - 1
end

local remaining = math.floor(tokens)
local reset = now
if rate > 0 then
  local refill = (capacity - tokens) / rate
  reset = now + refill
end

redis.call("HMSET", key, "tokens", tokens, "ts", ts)
redis.call("EXPIRE", key, ttl)

return { allowed, remaining, math.ceil(reset), capacity }
`
