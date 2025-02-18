package csvio

import (
	"encoding/csv"
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/brimdata/zed"
	"github.com/brimdata/zed/runtime/expr"
	"github.com/brimdata/zed/zcode"
	"github.com/brimdata/zed/zson"
)

var ErrNotDataFrame = errors.New("CSV output requires uniform records but multiple types encountered (consider 'fuse')")

type Writer struct {
	writer    io.WriteCloser
	encoder   *csv.Writer
	flattener *expr.Flattener
	first     *zed.TypeRecord
	strings   []string
}

type WriterOpts struct {
	UTF8 bool
}

func NewWriter(w io.WriteCloser) *Writer {
	return &Writer{
		writer:    w,
		encoder:   csv.NewWriter(w),
		flattener: expr.NewFlattener(zed.NewContext()),
	}
}

func (w *Writer) Close() error {
	w.encoder.Flush()
	return w.writer.Close()
}

func (w *Writer) Flush() error {
	w.encoder.Flush()
	return w.encoder.Error()
}

func (w *Writer) Write(rec *zed.Value) error {
	if rec.Type.Kind() != zed.RecordKind {
		return fmt.Errorf("CSV output encountered non-record value: %s", zson.MustFormatValue(rec))
	}
	rec, err := w.flattener.Flatten(rec)
	if err != nil {
		return err
	}
	if w.first == nil {
		w.first = zed.TypeRecordOf(rec.Type)
		var hdr []string
		for _, f := range rec.Fields() {
			hdr = append(hdr, f.Name)
		}
		if err := w.encoder.Write(hdr); err != nil {
			return err
		}
	} else if rec.Type != w.first {
		return ErrNotDataFrame
	}
	w.strings = w.strings[:0]
	fields := rec.Fields()
	for i, it := 0, rec.Bytes.Iter(); i < len(fields) && !it.Done(); i++ {
		var s string
		if zb := it.Next(); zb != nil {
			val := zed.NewValue(fields[i].Type, zb).Under()
			switch id := val.Type.ID(); {
			case id == zed.IDBytes && len(val.Bytes) == 0:
				// We want "" instead of "0x" for a zero-length value.
			case id == zed.IDString:
				s = string(val.Bytes)
			default:
				s = formatValue(val.Type, val.Bytes)
				if zed.IsFloat(id) && strings.HasSuffix(s, ".") {
					s = strings.TrimSuffix(s, ".")
				}
			}
		}
		w.strings = append(w.strings, s)
	}
	return w.encoder.Write(w.strings)
}

func formatValue(typ zed.Type, bytes zcode.Bytes) string {
	// Avoid ZSON decoration.
	if typ.ID() < zed.IDTypeComplex {
		return zson.FormatPrimitive(zed.TypeUnder(typ), bytes)
	}
	return zson.String(zed.Value{Type: typ, Bytes: bytes})
}
