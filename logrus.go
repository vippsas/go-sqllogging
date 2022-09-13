package sqllogging

import (
	"context"
	"database/sql"
	"fmt"
	"github.com/denisenkom/go-mssqldb/msdsn"
	"github.com/sirupsen/logrus"
	"io"
	"os"
	"regexp"
	"strings"
)

func logAtLevel(logger logrus.FieldLogger, level logrus.Level, msg string) {
	switch level {
	case logrus.DebugLevel:
		logger.Debug(msg)
	case logrus.InfoLevel:
		logger.Info(msg)
	case logrus.WarnLevel:
		logger.Warning(msg)
	case logrus.ErrorLevel:
		logger.Error(msg)
	default:
		// Note: This includes logrus.FatalLevel and logrus.PanicLevel;
		// which we do NOT support as aborting the process based on strings
		// from SQL in a log side-channel seems like a very bad idea.
		// We don't log those at Error either to avoid using this from
		// becoming widespread..
	}
}

type LogrusMssqlLogger interface {
	Log(ctx context.Context, logger logrus.FieldLogger, category msdsn.Log, msg string)
}

type QuerierExecer interface {
	QueryContext(ctx context.Context, query string, args ...interface{}) (*sql.Rows, error)
	QueryRowContext(ctx context.Context, query string, args ...interface{}) *sql.Row
	ExecContext(ctx context.Context, query string, args ...interface{}) (sql.Result, error)
}

// DB interface is used simply to avoid a potentially common mistake of passing
// a *Conn or *Tx to With()
type DB interface {
	QuerierExecer
	Conn(ctx context.Context) (*sql.Conn, error)
}

// StandardFallbackLogrusMssqlLogger defines the behaviour if no log level
// is specified with the "level:" prefix; i.e. messages that are likely
// not logged with this framework in mind.
type StandardFallbackLogrusMssqlLogger struct {
	Mask  msdsn.Log
	Level logrus.Level
}

func (s StandardFallbackLogrusMssqlLogger) Log(ctx context.Context, logger logrus.FieldLogger, category msdsn.Log, msg string) {
	if s.Mask&category != 0 {
		logAtLevel(logger, s.Level, msg)
	}
}

// With configures a standard opinionated logger, see LogrusLogger.
func With(ctx context.Context, logger logrus.FieldLogger, dbi DB, fallback ...LogrusMssqlLogger) context.Context {
	var f LogrusMssqlLogger
	switch len(fallback) {
	case 0:
		// Standard: Log SQL errors at logrus warning level
		f = StandardFallbackLogrusMssqlLogger{
			Mask:  msdsn.LogErrors,
			Level: logrus.WarnLevel,
		}
	case 1:
		f = fallback[0]
	default:
		panic("can only provide a single fallback")
	}
	return WithLogger(ctx, LogrusLogger{
		Logger:   logger,
		Querier:  dbi,
		Fallback: f,
		Stderr:   os.Stderr,
	})
}

// LogrusLogger is an opinionated logger implementation that parses the
// SQL log string and turns it into a nice logrus log; see README.md
// for further description.
type LogrusLogger struct {
	Logger   logrus.FieldLogger // Normal logrus output
	Querier  QuerierExecer      // For ##log-table dumping, this is used to fetch table data
	Fallback LogrusMssqlLogger  // If `<level>:` prefix is not present, forward to this logger
	Stderr   io.Writer          // The special "stderr:" level is written here
}

// For simplicty, only support a very restricted set of names for log tables..
var logTableNameRegexp = regexp.MustCompile(`^##[a-z0-9A-Z_]+$`)

func (l LogrusLogger) Log(ctx context.Context, category msdsn.Log, msg string) {

	if category&msdsn.LogMessages != 0 && strings.HasPrefix(msg, "Error: 50000") &&
		strings.Contains(msg, "The error is printed in terse mode because there was error during formatting") {
		// hack to help a common usage error..with bad error message from mssql...
		msg = "error:Wrong format string provided to formatmessage()"
	}

	logger := l.Logger
	level, logmsg, found := strings.Cut(msg, ":")
	if !found {
		level = ""
		logmsg = msg
	}

	// Support only a subset of logrus.ParseLevel, and in a stricter way,
	// + support special "" and "stderr" levels
	switch level {
	case "debug", "info", "warning", "error":
		var logrusLevel logrus.Level
		switch level {
		case "debug":
			logrusLevel = logrus.DebugLevel
		case "info":
			logrusLevel = logrus.InfoLevel
		case "warning":
			logrusLevel = logrus.WarnLevel
		case "error":
			logrusLevel = logrus.ErrorLevel
		}
		var fields logrus.Fields
		fields, logmsg = parseFields(logmsg)
		if fields != nil {
			logger = logger.WithFields(fields)
		}

		if l.Querier != nil && logTableNameRegexp.MatchString(logmsg) {
			tableDumpStructured(ctx, logger, logrusLevel, l.Querier, logmsg)
			dropTable(ctx, l.Querier, logmsg)
		} else {
			logAtLevel(logger, logrusLevel, logmsg)
		}
	case "stderr":
		if l.Querier != nil && logTableNameRegexp.MatchString(logmsg) {
			tableDumpPrettyPrint(ctx, l.Stderr, l.Querier, logmsg)
			dropTable(ctx, l.Querier, logmsg)
		} else {
			_, _ = fmt.Fprintln(l.Stderr, logmsg)
		}
	default:
		if level != "" {
			logmsg = level + ":" + logmsg
		}
		l.Fallback.Log(ctx, logger, category, msg)
	}
}

func dropTable(ctx context.Context, querier QuerierExecer, tablename string) {
	_, _ = querier.ExecContext(ctx, "drop table "+sqlQuotename(tablename))
}

func sqlQuotename(name string) string {
	return "[" + strings.ReplaceAll(name, "]", "]]") + "]"
}
