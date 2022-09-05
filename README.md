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

... = dbi.ExecContext(
	sqllogging.With(ctx, logger.WithField("inSql", true), dbi),
	`my_stored_procedure`, ...)
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
In MS SQL, if you create a table with a name starting
`##`, it can be accessed by other connections. Then,
if you log *only* the name of the table starting with `##`
then this library will open a second connection
to the DB to download the contents.
Example:

```sql
select top(100) a, b, c into ##log1 from mytable;

exec [code].log 'stderr', @table='##log1'
```

* The table should be a cross-section temporary `##`-table
  containing any data.
* The logging library will pick up the table name starting with
  `##` and open a 2nd connection to the database to fetch
  and log the contents, using the optional order by clause.
* After the log statement, the table should be left alone.
  The logging utility will `drop` it after logging it.
* The log query will be `select top(1000) * from ##log1 order by 1`;
  i.e. max 1000 rows and order by the 1st column.


This feature is the reason for passing the `*sql.DB` instance
to `sqllog.With()`. If you do not use this feature you may
safely pass `nil` instead, and `##log1` would be logged the
regular way.
