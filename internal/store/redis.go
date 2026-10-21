// Package store wraps the infrastructure Exam Shield depends on: Redis
// (locks, rate limits, caches, streams) and blob storage (MinIO or disk).
package store

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
)

// Redis bundles the client with an optional embedded server (standalone mode).
type Redis struct {
	*redis.Client
	embedded *miniredis.Miniredis
}

// OpenRedis connects to addr, or starts an embedded in-process Redis when addr
// is empty. The embedded server keeps standalone mode dependency-free; it is
// single-process and not durable, which is acceptable for locks/rate limits.
func OpenRedis(ctx context.Context, addr, password string, db int) (*Redis, error) {
	r := &Redis{}
	if addr == "" {
		mr := miniredis.NewMiniRedis()
		if err := mr.StartAddr("127.0.0.1:0"); err != nil {
			return nil, fmt.Errorf("start embedded redis: %w", err)
		}
		r.embedded = mr
		addr = mr.Addr()
	}
	r.Client = redis.NewClient(&redis.Options{
		Addr:         addr,
		Password:     password,
		DB:           db,
		DialTimeout:  3 * time.Second,
		ReadTimeout:  3 * time.Second,
		WriteTimeout: 3 * time.Second,
	})
	pingCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	if err := r.Ping(pingCtx).Err(); err != nil {
		r.Close()
		return nil, fmt.Errorf("redis ping %s: %w", addr, err)
	}
	return r, nil
}

// Embedded reports whether this is the in-process server.
func (r *Redis) Embedded() bool { return r.embedded != nil }

func (r *Redis) Close() error {
	err := r.Client.Close()
	if r.embedded != nil {
		r.embedded.Close()
	}
	return err
}

// --- Locks -------------------------------------------------------------------

// releaseScript deletes KEYS[1] only if it still holds ARGV[1], so a holder
// whose lock expired can never delete a lock someone else now owns.
var releaseScript = redis.NewScript(`
if redis.call("GET", KEYS[1]) == ARGV[1] then
  return redis.call("DEL", KEYS[1])
end
return 0`)

// refreshScript extends KEYS[1]'s TTL only if it still holds ARGV[1].
var refreshScript = redis.NewScript(`
if redis.call("GET", KEYS[1]) == ARGV[1] then
  return redis.call("PEXPIRE", KEYS[1], ARGV[2])
end
return 0`)

// claimPairScript atomically binds a device and a student to each other.
//
// KEYS[1] = session_lock:<device_fp>          (value: ARGV[1] = "<exam>:<student>")
// KEYS[2] = student_lock:<exam>:<student>     (value: ARGV[2] = device_fp)
// ARGV[3] = ttl ms
//
// Returns 1 on success, -1 if the device is bound to another student,
// -2 if the student is bound to another device.
var claimPairScript = redis.NewScript(`
local dev = redis.call("GET", KEYS[1])
if dev and dev ~= ARGV[1] then return -1 end
local stu = redis.call("GET", KEYS[2])
if stu and stu ~= ARGV[2] then return -2 end
redis.call("SET", KEYS[1], ARGV[1], "PX", ARGV[3])
redis.call("SET", KEYS[2], ARGV[2], "PX", ARGV[3])
return 1`)

var (
	ErrDeviceInUse  = errors.New("device already bound to another student")
	ErrStudentInUse = errors.New("student already active on another device")
)

func DeviceLockKey(deviceFP string) string { return "session_lock:" + deviceFP }
func StudentLockKey(examID, studentID int64) string {
	return fmt.Sprintf("student_lock:%d:%d", examID, studentID)
}
func pairValue(examID, studentID int64) string { return fmt.Sprintf("%d:%d", examID, studentID) }

// ClaimSession binds deviceFP <-> (exam, student) for ttl.
func (r *Redis) ClaimSession(ctx context.Context, deviceFP string, examID, studentID int64, ttl time.Duration) error {
	res, err := claimPairScript.Run(ctx, r.Client,
		[]string{DeviceLockKey(deviceFP), StudentLockKey(examID, studentID)},
		pairValue(examID, studentID), deviceFP, ttl.Milliseconds()).Int()
	if err != nil {
		return err
	}
	switch res {
	case -1:
		return ErrDeviceInUse
	case -2:
		return ErrStudentInUse
	}
	return nil
}

// RefreshSession extends both locks; returns false if either was lost.
func (r *Redis) RefreshSession(ctx context.Context, deviceFP string, examID, studentID int64, ttl time.Duration) (bool, error) {
	a, err := refreshScript.Run(ctx, r.Client, []string{DeviceLockKey(deviceFP)}, pairValue(examID, studentID), ttl.Milliseconds()).Int()
	if err != nil {
		return false, err
	}
	b, err := refreshScript.Run(ctx, r.Client, []string{StudentLockKey(examID, studentID)}, deviceFP, ttl.Milliseconds()).Int()
	if err != nil {
		return false, err
	}
	return a == 1 && b == 1, nil
}

// ReleaseSession drops both locks if this device still owns them.
func (r *Redis) ReleaseSession(ctx context.Context, deviceFP string, examID, studentID int64) error {
	if _, err := releaseScript.Run(ctx, r.Client, []string{DeviceLockKey(deviceFP)}, pairValue(examID, studentID)).Result(); err != nil {
		return err
	}
	_, err := releaseScript.Run(ctx, r.Client, []string{StudentLockKey(examID, studentID)}, deviceFP).Result()
	return err
}

// ForceReleaseStudent drops a student's locks regardless of owner (proctor unlock).
func (r *Redis) ForceReleaseStudent(ctx context.Context, examID, studentID int64) error {
	key := StudentLockKey(examID, studentID)
	fp, err := r.Get(ctx, key).Result()
	if err != nil && !errors.Is(err, redis.Nil) {
		return err
	}
	keys := []string{key}
	if fp != "" {
		keys = append(keys, DeviceLockKey(fp))
	}
	return r.Del(ctx, keys...).Err()
}

// --- Rate limiting -------------------------------------------------------------

// fixedWindowScript increments KEYS[1] and sets its TTL on first hit, in one
// round-trip, so a crash between INCR and EXPIRE cannot leave an immortal key.
var fixedWindowScript = redis.NewScript(`
local n = redis.call("INCR", KEYS[1])
if n == 1 then redis.call("PEXPIRE", KEYS[1], ARGV[1]) end
local ttl = redis.call("PTTL", KEYS[1])
return {n, ttl}`)

// Hit records one event against key and reports whether it is within limit,
// plus how long until the window resets.
func (r *Redis) Hit(ctx context.Context, key string, limit int, window time.Duration) (bool, time.Duration, error) {
	vals, err := fixedWindowScript.Run(ctx, r.Client, []string{key}, window.Milliseconds()).Int64Slice()
	if err != nil {
		return false, 0, err
	}
	return vals[0] <= int64(limit), time.Duration(vals[1]) * time.Millisecond, nil
}

// Once returns true the first time key is seen within ttl (dedup / cooldown).
func (r *Redis) Once(ctx context.Context, key string, ttl time.Duration) (bool, error) {
	return r.SetNX(ctx, key, 1, ttl).Result()
}
