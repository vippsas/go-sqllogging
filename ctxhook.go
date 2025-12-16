// Package sqllog provides a logging hook for go-mssqldb that turns mssqldb
// errors into proper logging using logrus. See README.md.
package sqllogging

import (
	"context"
	mssql "github.com/microsoft/go-mssqldb"
	"github.com/microsoft/go-mssqldb/msdsn"
)

type contextKey int

const ckLogger contextKey = iota

// WithLogger attaches an mssql.ContextLogger to ctx. This is the basic
// hook and you may use this directly to override the SQL logger per call.
func WithLogger(ctx context.Context, logger mssql.ContextLogger) context.Context {
	return context.WithValue(ctx, ckLogger, logger)
}

func LoggerOrNil(ctx context.Context) mssql.ContextLogger {
	val := ctx.Value(ckLogger)
	if val == nil {
		return nil
	}
	return val.(mssql.ContextLogger)
}

type mssqlLogHook struct{}

func InstallMssql() {
	mssql.SetContextLogger(mssqlLogHook{})
}

func (m mssqlLogHook) Log(ctx context.Context, category msdsn.Log, msg string) {
	logger := LoggerOrNil(ctx)
	if logger == nil {
		return
	}
	logger.Log(ctx, category, msg)
}
