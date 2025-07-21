//unused:disable
package mocks

import (
	"context"
	"database/sql"
)

// mockDB - мок базы данных для тестирования ошибок
// Реализует интерфейс DBExecutor
// Используется только в тестах
//go:generate mockgen -destination=db.go -package=mocks . DBExecutor

type MockDB struct {
	ExecContextFunc     func(ctx context.Context, query string, args ...interface{}) (sql.Result, error)
	QueryRowContextFunc func(ctx context.Context, query string, args ...interface{}) *sql.Row
	QueryContextFunc    func(ctx context.Context, query string, args ...interface{}) (*sql.Rows, error)
}

func (m *MockDB) ExecContext(ctx context.Context, query string, args ...interface{}) (sql.Result, error) {
	if m.ExecContextFunc != nil {
		return m.ExecContextFunc(ctx, query, args...)
	}
	return nil, nil
}

func (m *MockDB) QueryRowContext(ctx context.Context, query string, args ...interface{}) *sql.Row {
	if m.QueryRowContextFunc != nil {
		return m.QueryRowContextFunc(ctx, query, args...)
	}
	return nil
}

func (m *MockDB) QueryContext(ctx context.Context, query string, args ...interface{}) (*sql.Rows, error) {
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

// mockRow - мок строки результата
// Используется только в тестах
type mockRow struct {
	scanFunc func(dest ...interface{}) error
}

func (m *mockRow) Scan(dest ...interface{}) error {
	if m.scanFunc != nil {
		return m.scanFunc(dest...)
	}
	return nil
}

// mockRows - мок множественных строк результата
// Используется только в тестах
type mockRows struct {
	nextFunc  func() bool
	scanFunc  func(dest ...interface{}) error
	errFunc   func() error
	closeFunc func() error
}

func (m *mockRows) Next() bool {
	if m.nextFunc != nil {
		return m.nextFunc()
	}
	return false
}

func (m *mockRows) Scan(dest ...interface{}) error {
	if m.scanFunc != nil {
		return m.scanFunc(dest...)
	}
	return nil
}

func (m *mockRows) Err() error {
	if m.errFunc != nil {
		return m.errFunc()
	}
	return nil
}

func (m *mockRows) Close() error {
	if m.closeFunc != nil {
		return m.closeFunc()
	}
	return nil
}
