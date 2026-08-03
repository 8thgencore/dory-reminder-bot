package mocks

import (
	"context"
	"database/sql"
)

// mockDB - мок базы данных для тестирования ошибок
// Реализует интерфейс DBExecutor
// Используется только в тестах
type MockDB struct {
	ExecContextFunc     func(ctx context.Context, query string, args ...any) (sql.Result, error)
	QueryRowContextFunc func(ctx context.Context, query string, args ...any) *sql.Row
	QueryContextFunc    func(ctx context.Context, query string, args ...any) (*sql.Rows, error)
}

func (m *MockDB) ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error) {
	if m.ExecContextFunc != nil {
		return m.ExecContextFunc(ctx, query, args...)
	}
	return nil, nil
}

func (m *MockDB) QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row {
	if m.QueryRowContextFunc != nil {
		return m.QueryRowContextFunc(ctx, query, args...)
	}
	return nil
}

func (m *MockDB) QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error) {
	if m.QueryContextFunc != nil {
		return m.QueryContextFunc(ctx, query, args...)
	}
	return nil, nil
}

// MockResult - мок результата SQL запроса
// Реализует интерфейс sql.Result
// Используется только в тестах
type MockResult struct {
	LastInsertIDFunc func() (int64, error)
	RowsAffectedFunc func() (int64, error)
}

func (m *MockResult) LastInsertId() (int64, error) {
	if m.LastInsertIDFunc != nil {
		return m.LastInsertIDFunc()
	}
	return 0, nil
}

func (m *MockResult) RowsAffected() (int64, error) {
	if m.RowsAffectedFunc != nil {
		return m.RowsAffectedFunc()
	}
	return 0, nil
}
