package repository

import (
	"context"
	"database/sql"
)

// mockDB - мок базы данных для тестирования ошибок
// Реализует интерфейс DBExecutor
// Используется только в тестах
//go:generate mockgen -destination=mock_db.go -package=repository . DBExecutor

type mockDB struct {
	execContextFunc     func(ctx context.Context, query string, args ...interface{}) (sql.Result, error)
	queryRowContextFunc func(ctx context.Context, query string, args ...interface{}) *sql.Row
	queryContextFunc    func(ctx context.Context, query string, args ...interface{}) (*sql.Rows, error)
}

func (m *mockDB) ExecContext(ctx context.Context, query string, args ...interface{}) (sql.Result, error) {
	if m.execContextFunc != nil {
		return m.execContextFunc(ctx, query, args...)
	}
	return nil, nil
}

func (m *mockDB) QueryRowContext(ctx context.Context, query string, args ...interface{}) *sql.Row {
	if m.queryRowContextFunc != nil {
		return m.queryRowContextFunc(ctx, query, args...)
	}
	return nil
}

func (m *mockDB) QueryContext(ctx context.Context, query string, args ...interface{}) (*sql.Rows, error) {
	if m.queryContextFunc != nil {
		return m.queryContextFunc(ctx, query, args...)
	}
	return nil, nil
}

// mockResult - мок результата SQL запроса
// Реализует интерфейс sql.Result
// Используется только в тестах
type mockResult struct {
	lastInsertIDFunc func() (int64, error)
	rowsAffectedFunc func() (int64, error)
}

func (m *mockResult) LastInsertId() (int64, error) {
	if m.lastInsertIDFunc != nil {
		return m.lastInsertIDFunc()
	}
	return 0, nil
}

func (m *mockResult) RowsAffected() (int64, error) {
	if m.rowsAffectedFunc != nil {
		return m.rowsAffectedFunc()
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
