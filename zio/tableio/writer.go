package tableio

import (
	"fmt"
	"io"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/brimdata/zed"
	"github.com/brimdata/zed/runtime/expr"
	"github.com/brimdata/zed/zio/zeekio"
	"github.com/brimdata/zed/zson"
)

type Writer struct {
	writer    io.WriteCloser
	flattener *expr.Flattener
	table     *tabwriter.Writer
	typ       *zed.TypeRecord
	limit     int
	nline     int
}

func NewWriter(w io.WriteCloser) *Writer {
	table := tabwriter.NewWriter(w, 0, 8, 1, ' ', 0)
	return &Writer{
		writer:    w,
		flattener: expr.NewFlattener(zed.NewContext()),
		table:     table,
		limit:     1000,
	}
}

func (w *Writer) Write(r *zed.Value) error {
	if r.Type.Kind() != zed.RecordKind {
		return fmt.Errorf("table output encountered non-record value: %s", zson.MustFormatValue(r))
	}
	r, err := w.flattener.Flatten(r)
	if err != nil {
		return err
	}
	if r.Type != w.typ {
		if w.typ != nil {
			w.flush()
			w.nline = 0
		}
		// First time, or new descriptor, print header
		typ := zed.TypeRecordOf(r.Type)
		w.writeHeader(typ)
		w.typ = typ
	}
	if w.nline >= w.limit {
		w.flush()
		w.writeHeader(w.typ)
		w.nline = 0
	}
	var out []string
	for k, f := range r.Fields() {
		var v string
		value := r.DerefByColumn(k).MissingAsNull()
		if f.Type == zed.TypeTime {
			if !value.IsNull() {
				v = zed.DecodeTime(value.Bytes).Time().Format(time.RFC3339Nano)
			}
		} else {
			v = zeekio.FormatValue(value)
		}
		out = append(out, v)
	}
	w.nline++
	_, err = fmt.Fprintf(w.table, "%s\n", strings.Join(out, "\t"))
	return err
}

func (w *Writer) flush() error {
	return w.table.Flush()
}

func (w *Writer) writeHeader(typ *zed.TypeRecord) {
	for i, f := range typ.Fields {
		if i > 0 {
			w.table.Write([]byte{'\t'})
		}
		w.table.Write([]byte(f.Name))
	}
	w.table.Write([]byte{'\n'})
}

func (w *Writer) Close() error {
	err := w.flush()
	if closeErr := w.writer.Close(); err == nil {
		err = closeErr
	}
	return err
}
