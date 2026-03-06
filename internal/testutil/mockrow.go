package testutil

import (
	"fmt"
	"reflect"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

// StaticRow implements pgx.Row, copying pre-set values into Scan destinations.
type StaticRow struct {
	vals []interface{}
}

func NewRow(vals ...interface{}) pgx.Row {
	return &StaticRow{vals: vals}
}

func (r *StaticRow) Scan(dest ...any) error {
	if len(dest) != len(r.vals) {
		return fmt.Errorf("staticrow: expected %d dest, got %d", len(r.vals), len(dest))
	}
	for i, v := range r.vals {
		dv := reflect.ValueOf(dest[i])
		if dv.Kind() != reflect.Ptr {
			return fmt.Errorf("staticrow: dest[%d] is not a pointer", i)
		}
		sv := reflect.ValueOf(v)
		if !sv.Type().AssignableTo(dv.Elem().Type()) {
			// Try converting
			if sv.Type().ConvertibleTo(dv.Elem().Type()) {
				dv.Elem().Set(sv.Convert(dv.Elem().Type()))
				continue
			}
			return fmt.Errorf("staticrow: cannot assign %T to %T at index %d", v, dest[i], i)
		}
		dv.Elem().Set(sv)
	}
	return nil
}

// ErrorRow implements pgx.Row, always returning an error from Scan.
type ErrorRow struct {
	err error
}

func NewErrorRow(err error) pgx.Row {
	return &ErrorRow{err: err}
}

func (r *ErrorRow) Scan(dest ...any) error {
	return r.err
}

// StaticRows implements pgx.Rows for :many queries.
type StaticRows struct {
	data    [][]interface{}
	pos     int
	closed  bool
	scanErr error
}

func NewRows(data [][]interface{}) pgx.Rows {
	return &StaticRows{data: data, pos: -1}
}

func NewEmptyRows() pgx.Rows {
	return &StaticRows{data: nil, pos: -1}
}

func (r *StaticRows) Close()                              {}
func (r *StaticRows) Err() error                          { return r.scanErr }
func (r *StaticRows) CommandTag() pgconn.CommandTag       { return pgconn.NewCommandTag(fmt.Sprintf("SELECT %d", len(r.data))) }
func (r *StaticRows) FieldDescriptions() []pgconn.FieldDescription { return nil }
func (r *StaticRows) RawValues() [][]byte                 { return nil }
func (r *StaticRows) Conn() *pgx.Conn                    { return nil }
func (r *StaticRows) Values() ([]any, error)              { return nil, nil }

func (r *StaticRows) Next() bool {
	if r.closed {
		return false
	}
	r.pos++
	if r.pos >= len(r.data) {
		r.closed = true
		return false
	}
	return true
}

func (r *StaticRows) Scan(dest ...any) error {
	if r.pos < 0 || r.pos >= len(r.data) {
		return fmt.Errorf("staticrows: no current row")
	}
	row := r.data[r.pos]
	if len(dest) != len(row) {
		return fmt.Errorf("staticrows: expected %d dest, got %d", len(row), len(dest))
	}
	for i, v := range row {
		dv := reflect.ValueOf(dest[i])
		if dv.Kind() != reflect.Ptr {
			return fmt.Errorf("staticrows: dest[%d] is not a pointer", i)
		}
		sv := reflect.ValueOf(v)
		if !sv.Type().AssignableTo(dv.Elem().Type()) {
			if sv.Type().ConvertibleTo(dv.Elem().Type()) {
				dv.Elem().Set(sv.Convert(dv.Elem().Type()))
				continue
			}
			return fmt.Errorf("staticrows: cannot assign %T to %T at index %d", v, dest[i], i)
		}
		dv.Elem().Set(sv)
	}
	return nil
}
