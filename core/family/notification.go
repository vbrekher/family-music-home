package family

import (
	"context"
	"database/sql"

	"github.com/navidrome/navidrome/model/id"
)

func notify(ctx context.Context, tx *sql.Tx, user, entry, message, at string) error {
	_, e := tx.ExecContext(ctx, `INSERT INTO family_notification(id,user_id,entry_id,message,created_at) SELECT ?,?,?,?,? WHERE (SELECT count(*) FROM family_notification WHERE user_id=?)<10000`, id.NewRandom(), user, entry, message, at, user)
	return e
}
