package session

import (
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestManager_SetAndGet(t *testing.T) {
	sm := NewSessionManager()
	sm.Set(&AddReminderSession{ChatID: 1, UserID: 2, Step: StepTime, Type: "today"})

	got := sm.Get(1, 2)
	require.NotNil(t, got)
	assert.Equal(t, StepTime, got.Step)
	assert.Equal(t, "today", got.Type)

	assert.Nil(t, sm.Get(1, 999), "unknown user must have no session")
}

// Get отдаёт копию: правка результата не должна влиять на хранимое состояние,
// иначе параллельные обновления из одного чата гоняли бы одну структуру.
func TestManager_GetReturnsCopy(t *testing.T) {
	sm := NewSessionManager()
	sm.Set(&AddReminderSession{ChatID: 1, UserID: 2, Step: StepTime})

	first := sm.Get(1, 2)
	first.Step = StepConfirm
	first.Text = "подмена"

	second := sm.Get(1, 2)
	require.NotNil(t, second)
	assert.Equal(t, StepTime, second.Step)
	assert.Empty(t, second.Text)
}

func TestManager_Delete(t *testing.T) {
	sm := NewSessionManager()
	sm.Set(&AddReminderSession{ChatID: 1, UserID: 2})

	sm.Delete(1, 2)
	assert.Nil(t, sm.Get(1, 2))
}

func TestManager_ExpiresAbandonedSessions(t *testing.T) {
	sm := NewSessionManager()

	now := time.Now()
	sm.now = func() time.Time { return now }

	sm.Set(&AddReminderSession{ChatID: 1, UserID: 2, Step: StepTime})
	require.NotNil(t, sm.Get(1, 2))

	now = now.Add(sessionTTL + time.Second)
	assert.Nil(t, sm.Get(1, 2), "abandoned session must expire")
	assert.Zero(t, sm.Len(), "expired session must not linger in memory")
}

func TestManager_SetRefreshesTTL(t *testing.T) {
	sm := NewSessionManager()

	now := time.Now()
	sm.now = func() time.Time { return now }

	sm.Set(&AddReminderSession{ChatID: 1, UserID: 2, Step: StepTime})

	now = now.Add(sessionTTL - time.Minute)
	sm.Set(&AddReminderSession{ChatID: 1, UserID: 2, Step: StepText})

	now = now.Add(2 * time.Minute)
	got := sm.Get(1, 2)
	require.NotNil(t, got, "activity must extend the session")
	assert.Equal(t, StepText, got.Step)
}

// Обновления одного чата приходят в разных горутинах, поэтому менеджер обязан
// выдерживать конкурентный доступ.
func TestManager_ConcurrentAccess(t *testing.T) {
	sm := NewSessionManager()

	var wg sync.WaitGroup
	for i := range 50 {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			sm.Set(&AddReminderSession{ChatID: 1, UserID: 2, Step: StepTime, Interval: n})
			if got := sm.Get(1, 2); got != nil {
				got.Interval++
			}
			sm.Len()
		}(i)
	}
	wg.Wait()

	assert.NotNil(t, sm.Get(1, 2))
}
