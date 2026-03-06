package testutil

import (
	"context"
	"fmt"
	"sync"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

// MockDBTX implements store.DBTX with a FIFO queue of responses.
type MockDBTX struct {
	mu       sync.Mutex
	execQ    []execResult
	queryQ   []queryResult
	rowQ     []pgx.Row
	calls    []DBCall
}

type execResult struct {
	tag pgconn.CommandTag
	err error
}

type queryResult struct {
	rows pgx.Rows
	err  error
}

// DBCall records a call made to the mock.
type DBCall struct {
	Method string
	SQL    string
	Args   []interface{}
}

func NewMockDBTX() *MockDBTX {
	return &MockDBTX{}
}

// OnExec enqueues a result for the next Exec call.
func (m *MockDBTX) OnExec(tag pgconn.CommandTag, err error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.execQ = append(m.execQ, execResult{tag: tag, err: err})
}

// OnExecSuccess enqueues a successful Exec result.
func (m *MockDBTX) OnExecSuccess() {
	m.OnExec(pgconn.NewCommandTag("UPDATE 1"), nil)
}

// OnQuery enqueues a result for the next Query call.
func (m *MockDBTX) OnQuery(rows pgx.Rows, err error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.queryQ = append(m.queryQ, queryResult{rows: rows, err: err})
}

// OnQueryRow enqueues a Row for the next QueryRow call.
func (m *MockDBTX) OnQueryRow(row pgx.Row) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.rowQ = append(m.rowQ, row)
}

func (m *MockDBTX) Exec(ctx context.Context, sql string, args ...interface{}) (pgconn.CommandTag, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.calls = append(m.calls, DBCall{Method: "Exec", SQL: sql, Args: args})
	if len(m.execQ) == 0 {
		return pgconn.NewCommandTag(""), fmt.Errorf("mockdb: no Exec result queued")
	}
	r := m.execQ[0]
	m.execQ = m.execQ[1:]
	return r.tag, r.err
}

func (m *MockDBTX) Query(ctx context.Context, sql string, args ...interface{}) (pgx.Rows, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.calls = append(m.calls, DBCall{Method: "Query", SQL: sql, Args: args})
	if len(m.queryQ) == 0 {
		return nil, fmt.Errorf("mockdb: no Query result queued")
	}
	r := m.queryQ[0]
	m.queryQ = m.queryQ[1:]
	return r.rows, r.err
}

func (m *MockDBTX) QueryRow(ctx context.Context, sql string, args ...interface{}) pgx.Row {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.calls = append(m.calls, DBCall{Method: "QueryRow", SQL: sql, Args: args})
	if len(m.rowQ) == 0 {
		return NewErrorRow(fmt.Errorf("mockdb: no QueryRow result queued"))
	}
	r := m.rowQ[0]
	m.rowQ = m.rowQ[1:]
	return r
}

// ZeroCommandTag returns an empty command tag for error cases.
func ZeroCommandTag() pgconn.CommandTag {
	return pgconn.NewCommandTag("")
}

// Calls returns all recorded DB calls.
func (m *MockDBTX) Calls() []DBCall {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := make([]DBCall, len(m.calls))
	copy(out, m.calls)
	return out
}
