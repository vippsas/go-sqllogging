package sqllogging

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

func init() {
	InstallMssql()
}

func sqlOpen(t *testing.T) *sql.DB {
	dsn := os.Getenv("SQLSERVER_DSN")
	dsn = "sqlserver://localhost?database=master&user id=sa&password=VippsPw1"
	if dsn == "" {
		panic("Must set SQLSERVER_DSN to run tests")
	}
	dsn = dsn + "&log=63"
	dbi, err := sql.Open("sqlserver", dsn)
	require.NoError(t, err)
	return dbi
}

func TestLogrusLogging(t *testing.T) {
	dbi := sqlOpen(t)
	ctx := context.Background()

	var logbuf bytes.Buffer
	log := logrus.StandardLogger()
	log.Out = &logbuf
	log.Level = logrus.DebugLevel
	log.Formatter = &logrus.JSONFormatter{}
	t0, err := time.Parse(time.RFC3339, "2000-01-01T00:00:00Z")
	require.NoError(t, err)
	logger := log.WithField("intest", true).WithTime(t0)

	// Field parsing
	_, err = dbi.ExecContext(With(ctx, logger, nil), `
    declare @msg varchar(max);
    set @msg = concat('info:  a=1  nil=    b=', quotename('hey ]"/* ]] - ]]]'), ' hello world c=[3]')
	raiserror (@msg, 1, 0) with nowait;
`)
	require.NoError(t, err)
	assert.Equal(t,
		`{"a":1,"b":"hey ]\"/* ]] - ]]]","intest":true,"level":"info","msg":"hello world c=[3]","nil":null,"time":"2000-01-01T00:00:00Z"}`,
		strings.TrimSpace(logbuf.String()))

	logbuf.Reset()

	// Fallback logging; SQL error -> logrus warning
	_, err = dbi.ExecContext(With(ctx, logger, nil), `
	raiserror ('test 1', 0, 0) with nowait;
	raiserror ('test 2', 16, 0) with nowait;
	`)
	assert.True(t, strings.Contains(err.Error(), "test 2"))

	assert.Equal(t,
		`{"intest":true,"level":"warning","msg":"test 2","time":"2000-01-01T00:00:00Z"}`,
		strings.TrimSpace(logbuf.String()))

	// Structured table dumps

	// use a single Conn to be able to robustly assert that the
	// temp table is dropped
	conn, err := dbi.Conn(ctx)
	require.NoError(t, err)
	defer conn.Close()

	logbuf.Reset()
	_, err = conn.ExecContext(With(ctx, logger, dbi), `
	select x, concat('number ', x) as y into ##log1
	from (values (1), (2)) row(x)
	;
	raiserror ('debug:##log1', 0, 0) with nowait;
`)
	require.NoError(t, err)

	// ##-table should be dropped
	var dummy int
	err = conn.QueryRowContext(ctx, `select x from ##log1`).Scan(&dummy)
	assert.Contains(t, err.Error(), "Invalid object name '##log1'")
	assert.Equal(t, ``+
		`{"intest":true,"level":"debug","msg":"","time":"2000-01-01T00:00:00Z","x":1,"y":"number 1"}
{"intest":true,"level":"debug","msg":"","time":"2000-01-01T00:00:00Z","x":2,"y":"number 2"}
`, logbuf.String())
}

func TestStdErr(t *testing.T) {
	dbi := sqlOpen(t)
	ctx := context.Background()

	var logbuf, stderr bytes.Buffer

	log := logrus.StandardLogger()
	log.Out = &logbuf
	log.Formatter = &logrus.JSONFormatter{}

	qryCtx := WithLogger(ctx, LogrusLogger{
		Logger:   log,
		Querier:  dbi,
		Fallback: StandardFallbackLogrusMssqlLogger{},
		Stderr:   &stderr,
	})

	_, err := dbi.ExecContext(qryCtx, `
	raiserror ('stderr: a=1 b=[string] test 1', 1, 0) with nowait;
	raiserror ('info:test 2', 0, 0) with nowait;
	`)
	require.NoError(t, err)
	assert.Equal(t, " a=1 b=[string] test 1\n", stderr.String())

	// Dump table
	stderr.Reset()
	_, err = dbi.ExecContext(qryCtx, `
	raiserror ('stderr:before', 0, 0) with nowait;

	select x, concat('number ', x) as y into ##log1
	from (values (1), (2)) row(x)
	;
	raiserror ('stderr:##log1', 0, 0) with nowait;

	raiserror ('stderr:after', 0, 0) with nowait;
`)
	require.NoError(t, err)

	// NOTE: The test below is whitespace-sensitive
	assert.Equal(t, `before
================================
##log1
================================
x                   1               
y                   "number 1"      
----------------    ------------    
x                   2               
y                   "number 2"      
----------------    ------------    
after
`, stderr.String())

}

func TestMalformedPrintfRaiserror(t *testing.T) {
	t.Skip()
	return
	// Testbed for printf errors, not a regression test
	dbi := sqlOpen(t)
	ctx := context.Background()

	var logbuf bytes.Buffer
	log := logrus.StandardLogger()
	log.Out = &logbuf
	log.Formatter = &logrus.JSONFormatter{}

	_, err := dbi.ExecContext(ctx,
		`
create or alter procedure dbo.TestMalformedPrintfRaiserror
as begin
    begin try
        declare @msg nvarchar(max)
        declare @bad bigint = 24
        
		set @msg = formatmessage('info:test=%d hello world', @bad);
		raiserror (@msg, 0, 0) with nowait;
        
        -- This does not work very well, completely silenced:
        -- raiserror ('info:count=%d hello world', 0, 0, @bad) with nowait;
    end try
	begin catch
	    -- This catch block doesn't; but it silences errors with level < 10 so that the error
	    -- above isn't returned to caller... a typical case...
	    ;throw
    end catch
end
`)
	require.NoError(t, err)

	_, err = dbi.ExecContext(
		With(ctx, log, dbi),
		`dbo.TestMalformedPrintfRaiserror`,
	)
	require.NoError(t, err)
}
