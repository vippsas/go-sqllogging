// Package sql allows pulling in the SQL files present in this directory as a go library.
// The SQL files follows the conventions of the sqlcode tool:
// https://github.com/vippsas/sqlcode
package sql

import (
	"embed"
)

//go:embed *.sql
var SQLFS embed.FS
