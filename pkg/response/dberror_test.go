package response

import (
	"fmt"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

func TestDBError(t *testing.T) {
	gin.SetMode(gin.TestMode)
	for _, tc := range []struct {
		err  error
		want int
	}{
		{pgx.ErrNoRows, 404},
		{fmt.Errorf("get run: %w", pgx.ErrNoRows), 404},
		{&pgconn.PgError{Code: "23503"}, 400},
		{&pgconn.PgError{Code: "23514"}, 400},
		{&pgconn.PgError{Code: "23505"}, 409},
		{fmt.Errorf("boom"), 500},
	} {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		DBError(c, tc.err, "nf", "fail")
		if w.Code != tc.want {
			t.Errorf("%v: got %d want %d", tc.err, w.Code, tc.want)
		}
	}
}
