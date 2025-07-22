package repository

import (
	"context"
	"database/sql"
	"errors"
	"testing"
	"time"

	"github.com/8thgencore/dory-reminder-bot/internal/domain"
	"github.com/8thgencore/dory-reminder-bot/internal/repository/mocks"
	_ "github.com/mattn/go-sqlite3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// setupTestDB создает тестовую базу данных в памяти
func setupTestDB(t *testing.T) *sql.DB {
	db, err := sql.Open("sqlite3", ":memory:")
	require.NoError(t, err)

	// Создаем таблицу reminders
	_, err = db.Exec(`
		CREATE TABLE reminders (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			chat_id INTEGER NOT NULL,
			user_id INTEGER NOT NULL,
			text TEXT NOT NULL,
			next_time DATETIME NOT NULL,
			repeat INTEGER NOT NULL,
			repeat_days TEXT,
			repeat_every INTEGER,
			paused BOOLEAN NOT NULL,
			created_at DATETIME NOT NULL,
			updated_at DATETIME NOT NULL
		)
	`)
	require.NoError(t, err)

	return db
}

// createTestReminder создает тестовое напоминание
func createTestReminder() *domain.Reminder {
	now := time.Now()
	return &domain.Reminder{
		ChatID:      12345,
		UserID:      67890,
		Text:        "Test reminder",
		NextTime:    now.Add(time.Hour),
		Repeat:      domain.RepeatNone,
		RepeatDays:  []int{1, 2, 3},
		RepeatEvery: 0,
		Paused:      false,
		CreatedAt:   now,
		UpdatedAt:   now,
	}
}

func TestNewReminderRepository(t *testing.T) {
	t.Run("successful creation", func(t *testing.T) {
		db := setupTestDB(t)
		defer db.Close()

		repo := NewReminderRepository(db)
		assert.NotNil(t, repo)
	})

	t.Run("panic on nil db", func(t *testing.T) {
		assert.Panics(t, func() {
			NewReminderRepository(nil)
		})
	})
}

func TestReminderRepository_Create(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()
	repo := NewReminderRepository(db)

	t.Run("successful creation", func(t *testing.T) {
		rem := createTestReminder()

		err := repo.Create(context.Background(), rem)
		require.NoError(t, err)
		assert.Greater(t, rem.ID, int64(0))
	})

	t.Run("creation with zero timestamps", func(t *testing.T) {
		rem := createTestReminder()
		rem.CreatedAt = time.Time{}
		rem.UpdatedAt = time.Time{}

		err := repo.Create(context.Background(), rem)
		require.NoError(t, err)
		assert.Greater(t, rem.ID, int64(0))
		assert.False(t, rem.CreatedAt.IsZero())
		assert.False(t, rem.UpdatedAt.IsZero())
	})

	t.Run("creation with empty repeat days", func(t *testing.T) {
		rem := createTestReminder()
		rem.RepeatDays = nil

		err := repo.Create(context.Background(), rem)
		require.NoError(t, err)
		assert.Greater(t, rem.ID, int64(0))
	})

	t.Run("creation with negative chat ID (group)", func(t *testing.T) {
		rem := createTestReminder()
		rem.ChatID = -12345 // Группа

		err := repo.Create(context.Background(), rem)
		require.NoError(t, err)
		assert.Greater(t, rem.ID, int64(0))
	})

	t.Run("nil reminder", func(t *testing.T) {
		err := repo.Create(context.Background(), nil)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "reminder is nil")
	})

	t.Run("empty text", func(t *testing.T) {
		rem := createTestReminder()
		rem.Text = ""

		err := repo.Create(context.Background(), rem)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "reminder text cannot be empty")
	})

	t.Run("invalid chat ID", func(t *testing.T) {
		rem := createTestReminder()
		rem.ChatID = 0

		err := repo.Create(context.Background(), rem)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "invalid chat ID")
	})

	t.Run("invalid user ID", func(t *testing.T) {
		rem := createTestReminder()
		rem.UserID = -1

		err := repo.Create(context.Background(), rem)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "invalid user ID")
	})
}

func TestReminderRepository_GetByID(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()
	repo := NewReminderRepository(db)

	t.Run("successful retrieval", func(t *testing.T) {
		// Создаем напоминание
		rem := createTestReminder()
		err := repo.Create(context.Background(), rem)
		require.NoError(t, err)

		// Получаем напоминание
		retrieved, err := repo.GetByID(context.Background(), rem.ID)
		require.NoError(t, err)
		assert.Equal(t, rem.ID, retrieved.ID)
		assert.Equal(t, rem.ChatID, retrieved.ChatID)
		assert.Equal(t, rem.UserID, retrieved.UserID)
		assert.Equal(t, rem.Text, retrieved.Text)
		assert.Equal(t, rem.RepeatDays, retrieved.RepeatDays)
	})

	t.Run("not found", func(t *testing.T) {
		_, err := repo.GetByID(context.Background(), 99999)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "reminder with ID 99999 not found")
	})

	t.Run("invalid ID", func(t *testing.T) {
		_, err := repo.GetByID(context.Background(), 0)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "invalid reminder ID")
	})

	t.Run("negative ID", func(t *testing.T) {
		_, err := repo.GetByID(context.Background(), -1)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "invalid reminder ID")
	})
}

func TestReminderRepository_Update(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()
	repo := NewReminderRepository(db)

	t.Run("successful update", func(t *testing.T) {
		// Создаем напоминание
		rem := createTestReminder()
		err := repo.Create(context.Background(), rem)
		require.NoError(t, err)

		// Обновляем напоминание
		rem.Text = "Updated reminder"
		rem.RepeatDays = []int{4, 5, 6}

		err = repo.Update(context.Background(), rem)
		require.NoError(t, err)

		// Проверяем обновление
		retrieved, err := repo.GetByID(context.Background(), rem.ID)
		require.NoError(t, err)
		assert.Equal(t, "Updated reminder", retrieved.Text)
		assert.Equal(t, []int{4, 5, 6}, retrieved.RepeatDays)
	})

	t.Run("update non-existent", func(t *testing.T) {
		rem := createTestReminder()
		rem.ID = 99999

		err := repo.Update(context.Background(), rem)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "reminder with ID 99999 not found")
	})

	t.Run("invalid ID", func(t *testing.T) {
		rem := createTestReminder()
		rem.ID = 0

		err := repo.Update(context.Background(), rem)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "invalid reminder ID")
	})

	t.Run("nil reminder", func(t *testing.T) {
		err := repo.Update(context.Background(), nil)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "reminder is nil")
	})

	t.Run("empty text", func(t *testing.T) {
		rem := createTestReminder()
		err := repo.Create(context.Background(), rem)
		require.NoError(t, err)

		rem.Text = ""
		err = repo.Update(context.Background(), rem)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "reminder text cannot be empty")
	})
}

func TestReminderRepository_Delete(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()
	repo := NewReminderRepository(db)

	t.Run("successful deletion", func(t *testing.T) {
		// Создаем напоминание
		rem := createTestReminder()
		err := repo.Create(context.Background(), rem)
		require.NoError(t, err)

		// Удаляем напоминание
		err = repo.Delete(context.Background(), rem.ID)
		require.NoError(t, err)

		// Проверяем что напоминание удалено
		_, err = repo.GetByID(context.Background(), rem.ID)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "reminder with ID")
	})

	t.Run("delete non-existent", func(t *testing.T) {
		err := repo.Delete(context.Background(), 99999)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "reminder with ID 99999 not found")
	})

	t.Run("invalid ID", func(t *testing.T) {
		err := repo.Delete(context.Background(), 0)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "invalid reminder ID")
	})

	t.Run("negative ID", func(t *testing.T) {
		err := repo.Delete(context.Background(), -1)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "invalid reminder ID")
	})
}

func TestReminderRepository_ListByChat(t *testing.T) {
	t.Run("successful listing", func(t *testing.T) {
		db := setupTestDB(t)
		defer db.Close()
		repo := NewReminderRepository(db)

		chatID := int64(12345)

		// Создаем несколько напоминаний для одного чата
		rem1 := createTestReminder()
		rem1.ChatID = chatID
		rem1.Text = "First reminder"

		rem2 := createTestReminder()
		rem2.ChatID = chatID
		rem2.Text = "Second reminder"

		rem3 := createTestReminder()
		rem3.ChatID = 99999 // другой чат
		rem3.Text = "Other chat reminder"

		err := repo.Create(context.Background(), rem1)
		require.NoError(t, err)
		err = repo.Create(context.Background(), rem2)
		require.NoError(t, err)
		err = repo.Create(context.Background(), rem3)
		require.NoError(t, err)

		// Получаем напоминания для конкретного чата
		reminders, err := repo.ListByChat(context.Background(), chatID)
		require.NoError(t, err)
		assert.Len(t, reminders, 2)

		texts := make([]string, len(reminders))
		for i, r := range reminders {
			texts[i] = r.Text
		}
		assert.Contains(t, texts, "First reminder")
		assert.Contains(t, texts, "Second reminder")
	})

	t.Run("empty list", func(t *testing.T) {
		db := setupTestDB(t)
		defer db.Close()
		repo := NewReminderRepository(db)

		// Используем новый чат ID, который точно не существует
		reminders, err := repo.ListByChat(context.Background(), 999999)
		require.NoError(t, err)
		assert.Empty(t, reminders)
	})

	t.Run("invalid chat ID", func(t *testing.T) {
		db := setupTestDB(t)
		defer db.Close()
		repo := NewReminderRepository(db)

		_, err := repo.ListByChat(context.Background(), 0)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "invalid chat ID")
	})

	t.Run("negative chat ID", func(t *testing.T) {
		db := setupTestDB(t)
		defer db.Close()
		repo := NewReminderRepository(db)

		_, err := repo.ListByChat(context.Background(), -1)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "invalid chat ID")
	})
}

func TestReminderRepository_ListDue(t *testing.T) {
	t.Run("successful listing", func(t *testing.T) {
		db := setupTestDB(t)
		defer db.Close()
		repo := NewReminderRepository(db)

		now := time.Now()

		// Создаем напоминания с разными временами
		rem1 := createTestReminder()
		rem1.NextTime = now.Add(-time.Hour) // просроченное
		rem1.Paused = false

		rem2 := createTestReminder()
		rem2.NextTime = now.Add(time.Hour) // будущее
		rem2.Paused = false

		rem3 := createTestReminder()
		rem3.NextTime = now.Add(-30 * time.Minute) // просроченное
		rem3.Paused = true                         // приостановленное

		err := repo.Create(context.Background(), rem1)
		require.NoError(t, err)
		err = repo.Create(context.Background(), rem2)
		require.NoError(t, err)
		err = repo.Create(context.Background(), rem3)
		require.NoError(t, err)

		// Получаем просроченные напоминания
		reminders, err := repo.ListDue(context.Background(), now)
		require.NoError(t, err)
		assert.Len(t, reminders, 1) // только rem1, так как rem2 в будущем, а rem3 приостановлен
		assert.Equal(t, rem1.ID, reminders[0].ID)
	})

	t.Run("empty list", func(t *testing.T) {
		db := setupTestDB(t)
		defer db.Close()
		repo := NewReminderRepository(db)

		reminders, err := repo.ListDue(context.Background(), time.Now())
		require.NoError(t, err)
		assert.Empty(t, reminders)
	})

	t.Run("with zero time", func(t *testing.T) {
		db := setupTestDB(t)
		defer db.Close()
		repo := NewReminderRepository(db)

		reminders, err := repo.ListDue(context.Background(), time.Time{})
		require.NoError(t, err)
		assert.Empty(t, reminders)
	})
}

func TestSerializeRepeatDays(t *testing.T) {
	tests := []struct {
		name     string
		days     []int
		expected string
	}{
		{
			name:     "empty slice",
			days:     []int{},
			expected: "",
		},
		{
			name:     "nil slice",
			days:     nil,
			expected: "",
		},
		{
			name:     "single day",
			days:     []int{1},
			expected: "1",
		},
		{
			name:     "multiple days",
			days:     []int{1, 2, 3},
			expected: "1,2,3",
		},
		{
			name:     "mixed numbers",
			days:     []int{0, 7, 15},
			expected: "0,7,15",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := serializeRepeatDays(tt.days)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestDeserializeRepeatDays(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected []int
	}{
		{
			name:     "empty string",
			input:    "",
			expected: nil,
		},
		{
			name:     "single number",
			input:    "1",
			expected: []int{1},
		},
		{
			name:     "multiple numbers",
			input:    "1,2,3",
			expected: []int{1, 2, 3},
		},
		{
			name:     "with spaces",
			input:    " 1 , 2 , 3 ",
			expected: []int{1, 2, 3},
		},
		{
			name:     "with invalid numbers",
			input:    "1,abc,3,def",
			expected: []int{1, 3},
		},
		{
			name:     "mixed valid and invalid",
			input:    "1,,3,abc,5",
			expected: []int{1, 3, 5},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := deserializeRepeatDays(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestSplitAndTrim(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		sep      string
		expected []string
	}{
		{
			name:     "empty string",
			input:    "",
			sep:      ",",
			expected: []string{},
		},
		{
			name:     "single item",
			input:    "test",
			sep:      ",",
			expected: []string{"test"},
		},
		{
			name:     "multiple items",
			input:    "a,b,c",
			sep:      ",",
			expected: []string{"a", "b", "c"},
		},
		{
			name:     "with spaces",
			input:    " a , b , c ",
			sep:      ",",
			expected: []string{"a", "b", "c"},
		},
		{
			name:     "empty items",
			input:    "a,,c",
			sep:      ",",
			expected: []string{"a", "c"},
		},
		{
			name:     "only spaces",
			input:    "   ,  ,  ",
			sep:      ",",
			expected: []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := splitAndTrim(tt.input, tt.sep)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestValidateReminder(t *testing.T) {
	t.Run("valid reminder", func(t *testing.T) {
		rem := createTestReminder()
		err := validateReminder(rem)
		assert.NoError(t, err)
	})

	t.Run("nil reminder", func(t *testing.T) {
		err := validateReminder(nil)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "reminder is nil")
	})

	t.Run("empty text", func(t *testing.T) {
		rem := createTestReminder()
		rem.Text = ""
		err := validateReminder(rem)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "reminder text cannot be empty")
	})

	t.Run("invalid chat ID", func(t *testing.T) {
		rem := createTestReminder()
		rem.ChatID = 0
		err := validateReminder(rem)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "invalid chat ID")
	})

	t.Run("negative chat ID", func(t *testing.T) {
		rem := createTestReminder()
		rem.ChatID = -1
		err := validateReminder(rem)
		assert.NoError(t, err) // Отрицательные chatID валидны для групп
	})

	t.Run("invalid user ID", func(t *testing.T) {
		rem := createTestReminder()
		rem.UserID = 0
		err := validateReminder(rem)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "invalid user ID")
	})

	t.Run("negative user ID", func(t *testing.T) {
		rem := createTestReminder()
		rem.UserID = -1
		err := validateReminder(rem)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "invalid user ID")
	})
}

// Тест для проверки интеграции всех методов
func TestReminderRepository_Integration(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()
	repo := NewReminderRepository(db)

	// Создаем несколько напоминаний
	reminders := []*domain.Reminder{
		{
			ChatID:      123,
			UserID:      456,
			Text:        "First reminder",
			NextTime:    time.Now().Add(time.Hour),
			Repeat:      domain.RepeatEveryDay,
			RepeatDays:  []int{1, 2},
			RepeatEvery: 0,
			Paused:      false,
		},
		{
			ChatID:      123,
			UserID:      456,
			Text:        "Second reminder",
			NextTime:    time.Now().Add(-time.Hour), // просроченное
			Repeat:      domain.RepeatEveryWeek,
			RepeatDays:  []int{3, 4, 5},
			RepeatEvery: 0,
			Paused:      false,
		},
		{
			ChatID:      789,
			UserID:      101,
			Text:        "Other chat reminder",
			NextTime:    time.Now().Add(2 * time.Hour),
			Repeat:      domain.RepeatNone,
			RepeatDays:  nil,
			RepeatEvery: 0,
			Paused:      true,
		},
	}

	// Создаем все напоминания
	for _, rem := range reminders {
		err := repo.Create(context.Background(), rem)
		require.NoError(t, err)
		assert.Greater(t, rem.ID, int64(0))
	}

	// Проверяем получение по ID
	for _, rem := range reminders {
		retrieved, err := repo.GetByID(context.Background(), rem.ID)
		require.NoError(t, err)
		assert.Equal(t, rem.Text, retrieved.Text)
		assert.Equal(t, rem.ChatID, retrieved.ChatID)
		assert.Equal(t, rem.RepeatDays, retrieved.RepeatDays)
	}

	// Проверяем список по чату
	chatReminders, err := repo.ListByChat(context.Background(), 123)
	require.NoError(t, err)
	assert.Len(t, chatReminders, 2)

	// Проверяем просроченные напоминания
	dueReminders, err := repo.ListDue(context.Background(), time.Now())
	require.NoError(t, err)
	assert.Len(t, dueReminders, 1) // только второе напоминание

	// Обновляем первое напоминание
	reminders[0].Text = "Updated first reminder"
	reminders[0].RepeatDays = []int{6, 7}
	err = repo.Update(context.Background(), reminders[0])
	require.NoError(t, err)

	// Проверяем обновление
	updated, err := repo.GetByID(context.Background(), reminders[0].ID)
	require.NoError(t, err)
	assert.Equal(t, "Updated first reminder", updated.Text)
	assert.Equal(t, []int{6, 7}, updated.RepeatDays)

	// Удаляем третье напоминание
	err = repo.Delete(context.Background(), reminders[2].ID)
	require.NoError(t, err)

	// Проверяем что удалено
	_, err = repo.GetByID(context.Background(), reminders[2].ID)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not found")

	// Проверяем что остальные остались
	_, err = repo.GetByID(context.Background(), reminders[0].ID)
	assert.NoError(t, err)
	_, err = repo.GetByID(context.Background(), reminders[1].ID)
	assert.NoError(t, err)
}

func TestReminderRepository_Create_DatabaseErrors(t *testing.T) {
	t.Run("exec context error", func(t *testing.T) {
		mock := &mocks.MockDB{
			ExecContextFunc: func(ctx context.Context, query string, args ...interface{}) (sql.Result, error) {
				return nil, errors.New("database connection failed")
			},
		}

		repo := &reminderRepository{db: mock}
		rem := createTestReminder()

		err := repo.Create(context.Background(), rem)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "failed to create reminder")
	})

	t.Run("last insert ID error", func(t *testing.T) {
		mock := &mocks.MockDB{
			ExecContextFunc: func(ctx context.Context, query string, args ...interface{}) (sql.Result, error) {
				return &mocks.MockResult{
					LastInsertIDFunc: func() (int64, error) {
						return 0, errors.New("failed to get last insert ID")
					},
				}, nil
			},
		}

		repo := &reminderRepository{db: mock}
		rem := createTestReminder()

		err := repo.Create(context.Background(), rem)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "failed to get last insert ID")
	})
}

func TestReminderRepository_Update_DatabaseErrors(t *testing.T) {
	t.Run("exec context error", func(t *testing.T) {
		mock := &mocks.MockDB{
			ExecContextFunc: func(ctx context.Context, query string, args ...interface{}) (sql.Result, error) {
				return nil, errors.New("database connection failed")
			},
		}

		repo := &reminderRepository{db: mock}
		rem := createTestReminder()
		rem.ID = 1

		err := repo.Update(context.Background(), rem)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "failed to update reminder")
	})

	t.Run("rows affected error", func(t *testing.T) {
		mock := &mocks.MockDB{
			ExecContextFunc: func(ctx context.Context, query string, args ...interface{}) (sql.Result, error) {
				return &mocks.MockResult{
					RowsAffectedFunc: func() (int64, error) {
						return 0, errors.New("failed to get rows affected")
					},
				}, nil
			},
		}

		repo := &reminderRepository{db: mock}
		rem := createTestReminder()
		rem.ID = 1

		err := repo.Update(context.Background(), rem)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "failed to get rows affected")
	})
}

func TestReminderRepository_Delete_DatabaseErrors(t *testing.T) {
	t.Run("exec context error", func(t *testing.T) {
		mock := &mocks.MockDB{
			ExecContextFunc: func(ctx context.Context, query string, args ...interface{}) (sql.Result, error) {
				return nil, errors.New("database connection failed")
			},
		}

		repo := &reminderRepository{db: mock}

		err := repo.Delete(context.Background(), 1)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "failed to delete reminder")
	})

	t.Run("rows affected error", func(t *testing.T) {
		mock := &mocks.MockDB{
			ExecContextFunc: func(ctx context.Context, query string, args ...interface{}) (sql.Result, error) {
				return &mocks.MockResult{
					RowsAffectedFunc: func() (int64, error) {
						return 0, errors.New("failed to get rows affected")
					},
				}, nil
			},
		}

		repo := &reminderRepository{db: mock}

		err := repo.Delete(context.Background(), 1)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "failed to get rows affected")
	})
}

func TestReminderRepository_GetByID_DatabaseErrors(t *testing.T) {
	t.Run("query row context error", func(t *testing.T) {
		// Используем реальную in-memory базу, чтобы получить sql.ErrNoRows
		db, err := sql.Open("sqlite3", ":memory:")
		assert.NoError(t, err)
		defer db.Close()
		_, err = db.Exec(`CREATE TABLE reminders (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			chat_id INTEGER NOT NULL,
			user_id INTEGER NOT NULL,
			text TEXT NOT NULL,
			next_time DATETIME NOT NULL,
			repeat INTEGER NOT NULL,
			repeat_days TEXT,
			repeat_every INTEGER,
			paused BOOLEAN NOT NULL,
			created_at DATETIME NOT NULL,
			updated_at DATETIME NOT NULL
		)`)
		assert.NoError(t, err)

		repo := NewReminderRepository(db)
		_, err = repo.GetByID(context.Background(), 99999)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "not found")
	})
}

func TestReminderRepository_ListByChat_DatabaseErrors(t *testing.T) {
	t.Run("query context error", func(t *testing.T) {
		mock := &mocks.MockDB{
			QueryContextFunc: func(ctx context.Context, query string, args ...interface{}) (*sql.Rows, error) {
				return nil, errors.New("database connection failed")
			},
		}

		repo := &reminderRepository{db: mock}

		_, err := repo.ListByChat(context.Background(), 1)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "failed to query reminders by chat")
	})
}

func TestReminderRepository_ListDue_DatabaseErrors(t *testing.T) {
	t.Run("query context error", func(t *testing.T) {
		mock := &mocks.MockDB{
			QueryContextFunc: func(ctx context.Context, query string, args ...interface{}) (*sql.Rows, error) {
				return nil, errors.New("database connection failed")
			},
		}

		repo := &reminderRepository{db: mock}

		_, err := repo.ListDue(context.Background(), time.Now())
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "failed to query due reminders")
	})
}
