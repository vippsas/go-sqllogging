# This library has been deprecated

This library has been deprecated.  See [go-querysql](https://github.com/vippsas/go-querysql) instead; go-querysql also has logging (and now monitoring) capabilities.


# go-sqllogging: Tools for logging from SQL

When developing stored procedures it can be useful to log directly
to the service output. This package facilitates this for the
following combination:

* Go
* SQL driver [go-mssql](github.com/denisenkom/go-mssqldb)
* Logging library [logrus](github.com/sirupsen/logrus)

## Basic usage

```go
func init() {
	// Call mssql.SetContextLogger to install a hook
	// that gets the logging setup to use from the 
	// This same hook is also useful for other logging
	// libraries than logrus.
	sqllogging.InstallMssql()  
}

// Important to add "&log=3" to the connection DSN
// so that the SQL driver will attempt to log (and call the
// hook installed above).
// PS: Adjust this as appropriate depending on your DSN format etc
dsnWithLog := dsn + fmt.Sprintf("&log=%d", int(msdsn.LogMessages|msdsn.LogErrors))
sqlConnPool := sql.Open("sqlserver", dsnWithLog) 

// Finally, per call:
// 1) Set up a context with the attached logger *and* a link to the connection pool
sqlCtx := sqllogging.With(ctx, logger.WithField("inSql", true), sqlConnPool)
// 2) Execute using that context
... = sqlConnPool.ExecContext(sqlCtx, "my_stored_procedure", ...)
```

To use it from Microsoft SQL the underlying mechanism is `raiserror ... with nowait`.
So for instance the following will work fine:
```sql
raiserror ('info:stringfield=[hello world] intfield=3 nilfield= This is a test', 0, 0) with nowait;
```
Instead of using this interface directly we recommend the stored procedure
in [sql/mssql_logging.sql](sql/mssql_logging.sql). Then the same example
becomes:
```sql
exec [code].log 'info'
    , 'stringfield', 'hello world'
    , 'intfield', @myIntVar
    , 'nilfield', null
    , @msg = 'This is a test'
```
Unfortunately, there is no way in SQL to support expressions in stored procedure
calls, so only literals and variables are supported in this construct.

**Note:** The `print` command can use expressions, but does not
support `with nowait` option so that the log message is only sent over the
network after the full batch has completed running. Therefore this library
builds on `raiserror` instead which can provide immediate logging.

**Note:** There is a version of printf built into raiserror too that one
can use instead of `[code].log` or pre-concatenating strings. However,
using it is a bit tricky because any mistake you make (e.g., supply `%d`
instead of `%I64d` for a `bigint` parameter) the error is likely to be completely
silenced (whether it is depend on the context; the error level, stored procedure
or not, try-block or not, etc).

## Features

### Log levels

Log levels are specified at the beginning of the string
with one of `debug:`, `info:`, `warning:`, or `error:`.
Either of these will trigger logging on the specified level

If no log level is specified, the default is to use the
`warning` level; but this is configurable through an optional
fallback log handler.

Additionally, the log level `stderr:` writes directly to standard
output, not to the configured `logger`, in case this is useful
during debugging. This is not suitable for production code
(instead, configure a custom logger using `sqllogging.WithLogger`
that writes to stderr).

### Log fields and the string format

The "porcelain" command `[code].log()` will assemble the log string, so if you
use that you don't need to worry about the details:
```json
exec [code].log 'info',
    , 'numericfield', 1
    , 'stringfield', 'a string'
    @msg = 'Here comes the message. thisIs=NotAField.'
;
```

What happens under the hood:
Assuming a proper log level (not `stderr`) has been chosen,
fields will be parsed from the log string, like this:
```sql
raiserror ('info:numericfield=1 stringfield=[a string] Here comes the message. thisIs=NotAField.', 0, 0) with nowait;
```

Fields must be at the beginning of the string.
Numeric values are passed to logrus as `int`
Strings should be quoted with `[]`; `]]` is an escape for `]`
you can therefore use `quotename` to safely marshal any string:

```sql
declare @msg varchar(max);
set @msg = concat('info:', 'test=', quotename('id]"/'))
raiserror (@msg, 0, 0) with nowait;
```

### Dump tables

There is support for dumping table contents to logs.
The way this is done is (probably) not ready for high volume OLTP;
use it for slow batch jobs.

Example:

```sql
select top(100) a, b, c into #log1 from mytable;
exec [code].log 'stderr', @table='#log1'
drop table #log1
```

The table name is only used to communicate the data to `[code].log`
so it can be dropped afterwards (which is the caller's responsibility).

Under the hood, `[code].log` will copy the table to a new
table with a random name starting with `##`, and pass that name
to the this library as part of an error message. Then this library
will open a 2nd database connection to fetch data from this temporary
table, and drop it when it is done.

This feature is the reason for passing the `*sql.DB` instance
to `sqllog.With()`. If you do not use this feature you may
safely pass `nil` instead.
