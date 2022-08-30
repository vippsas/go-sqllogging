package sqllogging

import (
	"context"
	"database/sql"
	"fmt"
	"github.com/sirupsen/logrus"
	"io"
	"text/tabwriter"
	"time"

	"github.com/alecthomas/repr"
)

type Row []any

func scanRowsOfAny(rows *sql.Rows, next func(Row) error) error {
	types, err := rows.ColumnTypes()
	if err != nil {
		return err
	}
	n := len(types)
	rowValues := make([]interface{}, n, n)
	pointers := make([]interface{}, n, n)
	for i := 0; i < len(types); i++ {
		pointers[i] = &rowValues[i]
	}
	for rows.Next() {
		err = rows.Scan(pointers...)
		if err != nil {
			return err
		}

		var row Row
		for i, _ := range types {
			switch v := rowValues[i].(type) {
			case []uint8:
				row = append(row, string(v))
			case int64:
				// we don't really know that the query column is int64, that's just how all ints are returned,
				// and it's usually more convenient in tests with int
				row = append(row, int(v))
			case time.Time:
				row = append(row, v)
			default:
				row = append(row, v)
			}
		}
		err = next(row)
		if err != nil {
			return err
		}
	}
	return rows.Err()
}

func sqlQueryLogTable(tablename string) string {
	return "select top(1000) * from " + sqlQuotename(tablename) + " order by 1"
}

// Dump contents of table to stream in human-readable column form
func tableDumpPrettyPrint(ctx context.Context, w io.Writer, dbi QuerierExecer, tablename string) {
	_, _ = fmt.Fprintln(w, "================================")
	_, _ = fmt.Fprintln(w, tablename)
	_, _ = fmt.Fprintln(w, "================================")

	rows, err := dbi.QueryContext(ctx, sqlQueryLogTable(tablename))
	if err != nil {
		_, _ = fmt.Fprintln(w, err.Error())
		return
	}
	defer func() {
		err = rows.Close()
		if err != nil {
			_, _ = fmt.Fprintln(w, err.Error())
		}
		return
	}()

	tw := tabwriter.NewWriter(w, 0, 0, 4, ' ', 0)

	columns, err := rows.Columns()
	if err != nil {
		_, _ = fmt.Fprintln(w, err.Error())
		return
	}
	err = scanRowsOfAny(rows, func(row Row) error {
		for i, value := range row {
			var val interface{}
			switch v := value.(type) {
			case string:
				val = repr.String(v)
			default:
				val = v
			}
			_, _ = fmt.Fprintln(tw, fmt.Sprintf("%s\t%v\t", columns[i], val))
		}
		_, _ = fmt.Fprintln(tw, "----------------\t------------\t")
		return nil
	})

	if err != nil {
		_, _ = fmt.Fprintln(w, err.Error())
		return
	}
	_ = tw.Flush()
}

// Dump contents of table to logger
func tableDumpStructured(ctx context.Context, logger logrus.FieldLogger, level logrus.Level, dbi QuerierExecer, tablename string) {
	qry := sqlQueryLogTable(tablename)
	rows, err := dbi.QueryContext(ctx, qry)
	if err != nil {
		logger.Warning("Unable to log table " + tablename + ": " + err.Error())
		return
	}
	defer func() {
		err = rows.Close()
		if err != nil {
			logger.Warning("Problem in rows.Close() when logging table " + tablename + ": " + err.Error())
		}
		return
	}()

	columns, err := rows.Columns()
	if err != nil {
		logger.Warning("Problem in rows.Columns() when logging table " + tablename + ": " + err.Error())
		return
	}
	err = scanRowsOfAny(rows, func(row Row) error {
		fields := make(logrus.Fields)
		for i, value := range row {
			fields[columns[i]] = value
		}
		logAtLevel(logger.WithFields(fields), level, "")
		return nil
	})
	if err != nil {
		logger.Warning("Problem in scanning rows when logging table " + tablename + ": " + err.Error())
	}

}
