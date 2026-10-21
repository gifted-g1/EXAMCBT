package store

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newRedis(t *testing.T) *Redis {
	t.Helper()
	r, err := OpenRedis(context.Background(), "", "", 0)
	require.NoError(t, err)
	t.Cleanup(func() { r.Close() })
	return r
}

func TestClaimSession(t *testing.T) {
	ctx := context.Background()
	r := newRedis(t)
	ttl := time.Minute

	require.NoError(t, r.ClaimSession(ctx, "dev1", 1, 10, ttl))
	// Same pair re-claiming (page reload) is fine.
	require.NoError(t, r.ClaimSession(ctx, "dev1", 1, 10, ttl))
	// Another student on the same device is refused.
	assert.ErrorIs(t, r.ClaimSession(ctx, "dev1", 1, 11, ttl), ErrDeviceInUse)
	// The same student on another device is refused.
	assert.ErrorIs(t, r.ClaimSession(ctx, "dev2", 1, 10, ttl), ErrStudentInUse)

	ok, err := r.RefreshSession(ctx, "dev1", 1, 10, ttl)
	require.NoError(t, err)
	assert.True(t, ok)

	// A non-owner release must not delete the owner's locks.
	require.NoError(t, r.ReleaseSession(ctx, "dev2", 1, 10))
	assert.ErrorIs(t, r.ClaimSession(ctx, "dev2", 1, 10, ttl), ErrStudentInUse)

	require.NoError(t, r.ReleaseSession(ctx, "dev1", 1, 10))
	require.NoError(t, r.ClaimSession(ctx, "dev2", 1, 10, ttl))

	require.NoError(t, r.ForceReleaseStudent(ctx, 1, 10))
	require.NoError(t, r.ClaimSession(ctx, "dev3", 1, 10, ttl))
	ok, _ = r.RefreshSession(ctx, "dev2", 1, 10, ttl)
	assert.False(t, ok, "lost lock must be reported")
}

func TestHitFixedWindow(t *testing.T) {
	ctx := context.Background()
	r := newRedis(t)
	for i := 1; i <= 5; i++ {
		ok, _, err := r.Hit(ctx, "k", 5, time.Minute)
		require.NoError(t, err)
		assert.True(t, ok, "hit %d", i)
	}
	ok, retry, err := r.Hit(ctx, "k", 5, time.Minute)
	require.NoError(t, err)
	assert.False(t, ok)
	assert.Greater(t, retry, time.Duration(0))
	ttl := r.PTTL(ctx, "k").Val()
	assert.Greater(t, ttl, time.Duration(0), "key must always carry a TTL")
}

func TestStreamsRoundTrip(t *testing.T) {
	ctx := context.Background()
	r := newRedis(t)
	require.NoError(t, r.EnsureGroup(ctx, StreamFrames, GroupAIWorkers))
	require.NoError(t, r.EnsureGroup(ctx, StreamFrames, GroupAIWorkers), "idempotent")
	require.NoError(t, r.Publish(ctx, StreamFrames, FrameJob{Kind: "frame", SessionID: 9, Key: "frames/1/9/1.jpg"}))
	msgs, err := r.ReadGroup(ctx, StreamFrames, GroupAIWorkers, "c1", 10, 100*time.Millisecond, false)
	require.NoError(t, err)
	require.Len(t, msgs, 1)
	assert.Contains(t, string(msgs[0].Data), `"session_id":9`)

	// Unacked messages come back on the pending read.
	pending, err := r.ReadGroup(ctx, StreamFrames, GroupAIWorkers, "c1", 10, 0, true)
	require.NoError(t, err)
	require.Len(t, pending, 1)
	require.NoError(t, r.Ack(ctx, StreamFrames, GroupAIWorkers, pending[0].ID))
	pending, _ = r.ReadGroup(ctx, StreamFrames, GroupAIWorkers, "c1", 10, 0, true)
	assert.Empty(t, pending)
}

func TestDiskBlobs(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	b, err := OpenDisk(dir)
	require.NoError(t, err)
	require.NoError(t, b.Put(ctx, "frames/1/2/3.jpg", []byte("x"), "image/jpeg"))
	got, err := b.Get(ctx, "frames/1/2/3.jpg")
	require.NoError(t, err)
	assert.Equal(t, []byte("x"), got)

	for _, bad := range []string{"../etc/passwd", "/abs", "a//b", "a/../b", `a\b`, ""} {
		assert.Error(t, b.Put(ctx, bad, []byte("x"), ""), bad)
	}
	_, err = b.Get(ctx, "frames/1/2/nope.jpg")
	assert.ErrorIs(t, err, ErrBlobNotFound)

	require.NoError(t, b.Put(ctx, "frames/10/1/1.jpg", []byte("y"), ""))
	require.NoError(t, b.DeletePrefix(ctx, "frames/1/"))
	_, err = os.Stat(filepath.Join(dir, "frames", "1"))
	assert.True(t, os.IsNotExist(err))
	_, err = b.Get(ctx, "frames/10/1/1.jpg")
	assert.NoError(t, err, "prefix frames/1/ must not delete frames/10/")
}

func TestClaimStaleRecoversOrphans(t *testing.T) {
	ctx := context.Background()
	r := newRedis(t)
	require.NoError(t, r.EnsureGroup(ctx, StreamFindings, GroupAPI))
	require.NoError(t, r.Publish(ctx, StreamFindings, FindingsMsg{SessionID: 1}))
	// "api-old" reads and dies without ACK.
	msgs, err := r.ReadGroup(ctx, StreamFindings, GroupAPI, "api-old", 10, 10*time.Millisecond, false)
	require.NoError(t, err)
	require.Len(t, msgs, 1)
	time.Sleep(30 * time.Millisecond)
	got, err := r.ClaimStale(ctx, StreamFindings, GroupAPI, "api-new", 20*time.Millisecond, 10)
	require.NoError(t, err)
	require.Len(t, got, 1)
	assert.Equal(t, msgs[0].ID, got[0].ID)
}
