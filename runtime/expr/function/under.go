package function

import (
	"github.com/brimdata/zed"
)

// https://github.com/brimdata/zed/blob/main/docs/language/functions.md#under
type Under struct {
	zctx *zed.Context
}

func (u *Under) Call(ectx zed.Allocator, args []zed.Value) *zed.Value {
	val := args[0]
	switch typ := args[0].Type.(type) {
	case *zed.TypeNamed:
		return ectx.NewValue(typ.Type, val.Bytes)
	case *zed.TypeError:
		return ectx.NewValue(typ.Type, val.Bytes)
	case *zed.TypeUnion:
		return ectx.NewValue(typ.Untag(val.Bytes))
	case *zed.TypeOfType:
		t, err := u.zctx.LookupByValue(val.Bytes)
		if err != nil {
			return newError(u.zctx, ectx, err)
		}
		return u.zctx.LookupTypeValue(zed.TypeUnder(t))
	default:
		return &args[0]
	}
}
