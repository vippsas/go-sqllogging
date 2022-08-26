package sqllog

import (
	"context"
	"database/sql"
	"fmt"
	"github.com/denisenkom/go-mssqldb/msdsn"
	"github.com/sirupsen/logrus"
	"os"
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

type CtxQuerier interface {
	QueryContext(ctx context.Context, query string, args ...interface{}) (*sql.Rows, error)
	QueryRowContext(ctx context.Context, query string, args ...interface{}) *sql.Row
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
func With(ctx context.Context, logger logrus.FieldLogger, querier CtxQuerier, fallback ...LogrusMssqlLogger) context.Context {
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
		Querier:  querier,
		Fallback: f,
	})
}

// LogrusLogger is an opinionated logger implementation that parses the
// SQL log string and turns it into a nice logrus log; see README.md
// for further description.
type LogrusLogger struct {
	Logger   logrus.FieldLogger
	Querier  CtxQuerier
	Fallback LogrusMssqlLogger
}

func (l LogrusLogger) Log(ctx context.Context, category msdsn.Log, msg string) {
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
		logAtLevel(logger, logrusLevel, logmsg)
	case "stderr":
		_, _ = fmt.Fprintln(os.Stderr, logmsg)
	default:
		if level != "" {
			logmsg = level + ":" + logmsg
		}
		l.Fallback.Log(ctx, logger, category, msg)
	}
}
