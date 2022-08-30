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

To use it from Microsoft SQL the best way is `raiserror ... with nowait`
with priority 0; some generic examples:
```sql
raiserror ('info:This is a test', 0, 0) with nowait;

-- Log any string; need a separate variable
declare @msg varchar(max);
set @msg = concat('info:', 'test=', 'yes', ' ', 'This is a test')
raiserror (@msg, 0, 0) with nowait;

-- Use of printf built into raiserror:
raiserror ('test=%d count=%d This is a test', 0, 0, 1, 10) with nowait;
```

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
(instead, configure a custom logger using `sqllog.WithLogger`
that writes to stderr).

### Log fields

Assuming a proper log level (not `stderr`) has been chosen,
fields will be parsed from the log string, like this:
```sql
raiserror ('info:numericfield=1 stringfield=[a string] Here comes the mesage. thisIs=NotAField.', 0, 0) with nowait;
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

raiserror ('stderr:##log1', 0, 0) with nowait
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
