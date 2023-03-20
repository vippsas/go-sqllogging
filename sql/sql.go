package sql

import (
	"embed"
)

//go:embed *.sql
var SQLFS embed.FS
