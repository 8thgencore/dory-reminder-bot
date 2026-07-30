package authz

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	tele "gopkg.in/telebot.v4"
)

type stubChecker struct {
	role  tele.MemberStatus
	err   error
	calls atomic.Int64
}

func (s *stubChecker) ChatMemberOf(_, _ tele.Recipient) (*tele.ChatMember, error) {
	s.calls.Add(1)
	if s.err != nil {
		return nil, s.err
	}

	return &tele.ChatMember{Role: s.role}, nil
}

const (
	userID  = int64(42)
	groupID = int64(-1001234567890)
)

func TestCheck_PrivateChatNeedsNoAPICall(t *testing.T) {
	checker := &stubChecker{role: tele.Left}
	a := New(checker)

	require.NoError(t, a.Check(context.Background(), userID, userID))
	assert.Zero(t, checker.calls.Load(), "private chat must not hit the Bot API")
}

func TestCheck_RolesDecideAccess(t *testing.T) {
	tests := []struct {
		role    tele.MemberStatus
		allowed bool
	}{
		{tele.Creator, true},
		{tele.Administrator, true},
		{tele.Member, true},
		{tele.Restricted, true},
		{tele.Left, false},
		{tele.Kicked, false},
		{tele.MemberStatus("unknown"), false},
	}

	for _, tt := range tests {
		t.Run(string(tt.role), func(t *testing.T) {
			a := New(&stubChecker{role: tt.role})

			err := a.Check(context.Background(), userID, groupID)
			if tt.allowed {
				assert.NoError(t, err)
			} else {
				assert.ErrorIs(t, err, ErrForbidden)
			}
		})
	}
}

// Сетевой сбой не должен открывать доступ и не должен попадать в кэш.
func TestCheck_APIErrorDeniesAndIsNotCached(t *testing.T) {
	checker := &stubChecker{err: errors.New("telegram is unreachable")}
	a := New(checker)

	assert.ErrorIs(t, a.Check(context.Background(), userID, groupID), ErrForbidden)
	assert.ErrorIs(t, a.Check(context.Background(), userID, groupID), ErrForbidden)
	assert.EqualValues(t, 2, checker.calls.Load(), "failures must not be cached")
}

func TestCheck_RechecksAllowedMembership(t *testing.T) {
	checker := &stubChecker{role: tele.Member}
	a := New(checker)

	require.NoError(t, a.Check(context.Background(), userID, groupID))
	checker.role = tele.Left

	assert.ErrorIs(t, a.Check(context.Background(), userID, groupID), ErrForbidden)
	assert.EqualValues(t, 2, checker.calls.Load(), "allowed membership must not be cached")
}

func TestCheck_CachesDenial(t *testing.T) {
	checker := &stubChecker{role: tele.Left}
	a := New(checker)

	for range 5 {
		assert.ErrorIs(t, a.Check(context.Background(), userID, groupID), ErrForbidden)
	}
	assert.EqualValues(t, 1, checker.calls.Load())
}

func TestCheck_CacheExpires(t *testing.T) {
	checker := &stubChecker{role: tele.Left}
	a := New(checker)

	now := time.Now()
	a.now = func() time.Time { return now }

	assert.ErrorIs(t, a.Check(context.Background(), userID, groupID), ErrForbidden)
	require.EqualValues(t, 1, checker.calls.Load())

	now = now.Add(defaultCacheTTL + time.Second)
	assert.ErrorIs(t, a.Check(context.Background(), userID, groupID), ErrForbidden)
	assert.EqualValues(t, 2, checker.calls.Load(), "expired entry must be refetched")
}

func TestForget_DropsCachedDecision(t *testing.T) {
	checker := &stubChecker{role: tele.Left}
	a := New(checker)

	assert.ErrorIs(t, a.Check(context.Background(), userID, groupID), ErrForbidden)
	a.Forget(userID, groupID)
	assert.ErrorIs(t, a.Check(context.Background(), userID, groupID), ErrForbidden)

	assert.EqualValues(t, 2, checker.calls.Load())
}

func TestCheck_RejectsZeroIDs(t *testing.T) {
	a := New(&stubChecker{role: tele.Member})

	assert.ErrorIs(t, a.Check(context.Background(), 0, groupID), ErrForbidden)
	assert.ErrorIs(t, a.Check(context.Background(), userID, 0), ErrForbidden)
}

// Отмена контекста не должна блокировать обработчик на таймауте Bot API.
func TestCheck_HonoursContextCancellation(t *testing.T) {
	blocked := make(chan struct{})
	t.Cleanup(func() { close(blocked) })

	a := New(&blockingChecker{release: blocked})

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	assert.ErrorIs(t, a.Check(ctx, userID, groupID), ErrForbidden)
}

type blockingChecker struct{ release chan struct{} }

func (b *blockingChecker) ChatMemberOf(_, _ tele.Recipient) (*tele.ChatMember, error) {
	<-b.release

	return &tele.ChatMember{Role: tele.Member}, nil
}
