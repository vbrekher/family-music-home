package persistence

import (
	"fmt"
	"github.com/navidrome/navidrome/model"
	"github.com/pocketbase/dbx"
)

// WithFamilyTransaction keeps normal encrypted-account writes, canonical library
// grants and additive family policy in the same existing SQLite transaction.
func WithFamilyTransaction(ds model.DataStore, block func(model.DataStore, dbx.Builder) error) error {
	return ds.WithTxImmediate(func(tx model.DataStore) error {
		sql, ok := tx.(*SQLStore)
		if !ok {
			return fmt.Errorf("SQL transaction required")
		}
		return block(tx, sql.db)
	}, "family-policy")
}
