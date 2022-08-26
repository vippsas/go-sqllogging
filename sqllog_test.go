package sqllog

import (
	"bytes"
	"context"
	"database/sql"
	"github.com/stretchr/testify/assert"
	"os"
	"strings"
	"testing"
	"time"

	_ "github.com/denisenkom/go-mssqldb"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/require"
)

func TestLogrusLogging(t *testing.T) {
	InstallMssql()

	dsn := os.Getenv("SQLSERVER_DSN")
	dsn = "sqlserver://localhost?database=master&user id=sa&password=VippsPw1"
	if dsn == "" {
		panic("Must set SQLSERVER_DSN to run tests")
	}
	dsn = dsn + "&log=63"

	ctx := context.Background()
	dbi, err := sql.Open("sqlserver", dsn)
	require.NoError(t, err)

	var buf bytes.Buffer
	log := logrus.StandardLogger()
	log.Out = &buf
	log.Formatter = &logrus.JSONFormatter{}
	t0, err := time.Parse(time.RFC3339, "2000-01-01T00:00:00Z")
	require.NoError(t, err)
	logger := log.WithField("intest", true).WithTime(t0)

	// Field parsing
	_, err = dbi.ExecContext(With(ctx, logger, nil), `
    declare @msg varchar(max);
    set @msg = concat('info:  a=1      b=', quotename('hey ]"/* ]] - ]]]'), ' hello world c=[3]')
	raiserror (@msg, 1, 0) with nowait;
`)
	require.NoError(t, err)
	assert.Equal(t,
		`{"a":1,"b":"hey ]\"/* ]] - ]]]","intest":true,"level":"info","msg":"hello world c=[3]","time":"2000-01-01T00:00:00Z"}`,
		strings.TrimSpace(buf.String()))

	buf.Reset()

	// Fallback logging; SQL error -> logrus warning
	_, err = dbi.ExecContext(With(ctx, logger, nil), `
	raiserror ('test 1', 1, 0) with nowait;
	raiserror ('test 2', 16, 0) with nowait;
`)
	assert.Equal(t,
		`{"intest":true,"level":"warning","msg":"test 2","time":"2000-01-01T00:00:00Z"}`,
		strings.TrimSpace(buf.String()))

}
