package limiter

import "github.com/redis/go-redis/v9"

// A Lua script to ensure an atomic implementation of the sliding window algorithim in Redis.
// First, old requests outside the sliding window are removed.
// Second, the number of current requests within the sliding window are counted.
// Finally, if the count is below the limit, the request is added to the user's sorted set,
// and 0 is returned, otherwise, 1 is returned.
var slidingWindowScript = redis.NewScript(`
    local key = KEYS[1]
    local now = tonumber(ARGV[1])
    local window = tonumber(ARGV[2])
    local limit = tonumber(ARGV[3])
    local clearBefore = now - window

    redis.call("ZREMRANGEBYSCORE", key, 0, clearBefore)

    local currentCount = redis.call("ZCARD", key)

    if currentCount < limit then
        redis.call("ZADD", key, now, ARGV[4])
        redis.call("PEXPIRE", key, window)
        return 0
    else
        return 1
    end
`)
