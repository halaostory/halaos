package access

import (
	"reflect"
	"testing"
)

func TestEmployeeRefs(t *testing.T) {
	for _, tc := range []struct {
		in   string
		want []int64
	}{
		{`{"employee_id": 7}`, []int64{7}},
		{`{"employee_ids": [1, 2, 0]}`, []int64{1, 2}},
		{`{"employee_id": 3, "employee_ids": [4]}`, []int64{3, 4}},
		{`{"employee_id": null}`, nil},
		{`{"name": "x"}`, nil},
		{`not json`, nil},
	} {
		if got := employeeRefs([]byte(tc.in)); !reflect.DeepEqual(got, tc.want) {
			t.Errorf("%s: got %v want %v", tc.in, got, tc.want)
		}
	}
}
