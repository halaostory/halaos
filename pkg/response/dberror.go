package response

import (
	"errors"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

// DBError answers a failed query with the right status instead of a
// blanket 500: a missing row is 404 (notFound), a broken reference or
// check is 400, a duplicate is 409. Anything else is a 500 with msg.
func DBError(c *gin.Context, err error, notFound, msg string) {
	if errors.Is(err, pgx.ErrNoRows) {
		NotFound(c, notFound)
		return
	}
	var pg *pgconn.PgError
	if errors.As(err, &pg) {
		switch pg.Code {
		case "23503": // foreign_key_violation
			BadRequest(c, "A referenced record does not exist")
			return
		case "23514", "22P02", "22007", "22008", "22003": // check, invalid text/datetime, out of range
			BadRequest(c, "Invalid value")
			return
		case "23505": // unique_violation
			Conflict(c, "This already exists")
			return
		}
	}
	InternalError(c, msg)
}
