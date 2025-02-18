package parquetio

import (
	"errors"
	"fmt"
	"math"

	"github.com/brimdata/zed"
	"github.com/fraugster/parquet-go/parquet"
	"github.com/fraugster/parquet-go/parquetschema"
)

var (
	ErrEmptyRecordType = errors.New("empty record type unsupported")
	ErrNullType        = errors.New("null type unimplemented")
	ErrUnionType       = errors.New("union type unsupported")
)

var (
	repetitionRequired = parquet.FieldRepetitionTypePtr(parquet.FieldRepetitionType_REQUIRED)
	repetitionOptional = parquet.FieldRepetitionTypePtr(parquet.FieldRepetitionType_OPTIONAL)
	repetitionRepeated = parquet.FieldRepetitionTypePtr(parquet.FieldRepetitionType_REPEATED)

	convertedUTF8            = parquet.ConvertedTypePtr(parquet.ConvertedType_UTF8)
	convertedMap             = parquet.ConvertedTypePtr(parquet.ConvertedType_MAP)
	convertedMapKeyValue     = parquet.ConvertedTypePtr(parquet.ConvertedType_MAP_KEY_VALUE)
	convertedList            = parquet.ConvertedTypePtr(parquet.ConvertedType_LIST)
	convertedEnum            = parquet.ConvertedTypePtr(parquet.ConvertedType_ENUM)
	convertedDate            = parquet.ConvertedTypePtr(parquet.ConvertedType_DATE)
	convertedTimeMillis      = parquet.ConvertedTypePtr(parquet.ConvertedType_TIME_MILLIS)
	convertedTimeMicros      = parquet.ConvertedTypePtr(parquet.ConvertedType_TIME_MICROS)
	convertedTimestampMillis = parquet.ConvertedTypePtr(parquet.ConvertedType_TIMESTAMP_MILLIS)
	convertedTimestampMicros = parquet.ConvertedTypePtr(parquet.ConvertedType_TIMESTAMP_MICROS)
	convertedUint8           = parquet.ConvertedTypePtr(parquet.ConvertedType_UINT_8)
	convertedUint16          = parquet.ConvertedTypePtr(parquet.ConvertedType_UINT_16)
	convertedUint32          = parquet.ConvertedTypePtr(parquet.ConvertedType_UINT_32)
	convertedUint64          = parquet.ConvertedTypePtr(parquet.ConvertedType_UINT_64)
	convertedInt8            = parquet.ConvertedTypePtr(parquet.ConvertedType_INT_8)
	convertedInt16           = parquet.ConvertedTypePtr(parquet.ConvertedType_INT_16)
	convertedInt32           = parquet.ConvertedTypePtr(parquet.ConvertedType_INT_32)
	convertedInt64           = parquet.ConvertedTypePtr(parquet.ConvertedType_INT_64)
	convertedJSON            = parquet.ConvertedTypePtr(parquet.ConvertedType_JSON)
	convertedBSON            = parquet.ConvertedTypePtr(parquet.ConvertedType_BSON)
	convertedInterval        = parquet.ConvertedTypePtr(parquet.ConvertedType_INTERVAL)

	logicalString          = &parquet.LogicalType{STRING: &parquet.StringType{}}
	logicalMap             = &parquet.LogicalType{MAP: &parquet.MapType{}}
	logicalList            = &parquet.LogicalType{LIST: &parquet.ListType{}}
	logicalEnum            = &parquet.LogicalType{ENUM: &parquet.EnumType{}}
	logicalDate            = &parquet.LogicalType{DATE: &parquet.DateType{}}
	logicalTimeMillis      = &parquet.LogicalType{TIME: &parquet.TimeType{Unit: timeUnitMillis}}
	logicalTimeMicros      = &parquet.LogicalType{TIME: &parquet.TimeType{Unit: timeUnitMicros}}
	logicalTimeNanos       = &parquet.LogicalType{TIME: &parquet.TimeType{Unit: timeUnitNanos}}
	logicalTimestampMillis = &parquet.LogicalType{TIMESTAMP: &parquet.TimestampType{Unit: timeUnitMillis}}
	logicalTimestampMicros = &parquet.LogicalType{TIMESTAMP: &parquet.TimestampType{Unit: timeUnitMicros}}
	logicalTimestampNanos  = &parquet.LogicalType{TIMESTAMP: &parquet.TimestampType{Unit: timeUnitNanos}}
	logicalUint8           = &parquet.LogicalType{INTEGER: &parquet.IntType{BitWidth: 8}}
	logicalUint16          = &parquet.LogicalType{INTEGER: &parquet.IntType{BitWidth: 16}}
	logicalUint32          = &parquet.LogicalType{INTEGER: &parquet.IntType{BitWidth: 32}}
	logicalUint64          = &parquet.LogicalType{INTEGER: &parquet.IntType{BitWidth: 64}}
	logicalInt8            = &parquet.LogicalType{INTEGER: &parquet.IntType{BitWidth: 8, IsSigned: true}}
	logicalInt16           = &parquet.LogicalType{INTEGER: &parquet.IntType{BitWidth: 16, IsSigned: true}}
	logicalInt32           = &parquet.LogicalType{INTEGER: &parquet.IntType{BitWidth: 32, IsSigned: true}}
	logicalInt64           = &parquet.LogicalType{INTEGER: &parquet.IntType{BitWidth: 64, IsSigned: true}}
	logicalBSON            = &parquet.LogicalType{BSON: &parquet.BsonType{}}
	logicalJSON            = &parquet.LogicalType{JSON: &parquet.JsonType{}}
	logicalUUID            = &parquet.LogicalType{UUID: &parquet.UUIDType{}}

	timeUnitMillis = &parquet.TimeUnit{MILLIS: &parquet.MilliSeconds{}}
	timeUnitMicros = &parquet.TimeUnit{MICROS: &parquet.MicroSeconds{}}
	timeUnitNanos  = &parquet.TimeUnit{NANOS: &parquet.NanoSeconds{}}
)

func newSchemaDefinition(typ *zed.TypeRecord) (*parquetschema.SchemaDefinition, error) {
	c, err := newColumnDefinition("", typ)
	if err != nil {
		return nil, err
	}
	s := &parquetschema.SchemaDefinition{
		RootColumn: &parquetschema.ColumnDefinition{
			Children: c.Children,
			SchemaElement: &parquet.SchemaElement{
				Name: "zq",
			},
		},
	}
	return s, s.ValidateStrict()
}

func newColumnDefinition(name string, typ zed.Type) (*parquetschema.ColumnDefinition, error) {
	switch typ := typ.(type) {
	case *zed.TypeNamed:
		switch id := typ.Type.ID(); {
		case typ.Name == "date" && id == zed.IDInt32:
			return newPrimitiveColumnDefinition(name, parquet.Type_INT32, convertedDate, logicalDate)
		case typ.Name == "bson" && id == zed.IDBytes:
			return newPrimitiveColumnDefinition(name, parquet.Type_BYTE_ARRAY, convertedBSON, logicalBSON)
		case typ.Name == "interval" && id == zed.IDBytes:
			return newPrimitiveColumnDefinition(name, parquet.Type_BYTE_ARRAY, convertedInterval, nil)
		case typ.Name == "json" && id == zed.IDString:
			return newPrimitiveColumnDefinition(name, parquet.Type_BYTE_ARRAY, convertedJSON, logicalJSON)
		case typ.Name == "enum" && id == zed.IDString:
			return newColumnDefinition(name, &zed.TypeEnum{})
		case typ.Name == "float" && id == zed.IDFloat64:
			return newPrimitiveColumnDefinition(name, parquet.Type_FLOAT, nil, nil)
		case typ.Name == "int96" && id == zed.IDBytes:
			return newPrimitiveColumnDefinition(name, parquet.Type_INT96, nil, nil)
		case typ.Name == "time_millis" && id == zed.IDInt32:
			return newPrimitiveColumnDefinition(
				name, parquet.Type_INT32, convertedTimeMillis, logicalTimeMillis)
		case name == "time_micros" && id == zed.IDInt64:
			return newPrimitiveColumnDefinition(
				name, parquet.Type_INT64, convertedTimeMicros, logicalTimeMicros)
		case name == "time_nanos" && id == zed.IDInt64:
			return newPrimitiveColumnDefinition(name, parquet.Type_INT64, nil, logicalTimeNanos)
		case name == "timestamp_millis" && id == zed.IDInt64:
			return newPrimitiveColumnDefinition(
				name, parquet.Type_INT64, convertedTimestampMillis, logicalTimestampMillis)
		case name == "timestamp_micros" && id == zed.IDInt64:
			return newPrimitiveColumnDefinition(
				name, parquet.Type_INT64, convertedTimestampMicros, logicalTimestampMicros)
		case name == "uuid" && id == zed.IDBytes:
			return newPrimitiveColumnDefinition(name, parquet.Type_BYTE_ARRAY, nil, logicalUUID)
		}
		return newColumnDefinition(name, typ.Type)
	case *zed.TypeOfUint8:
		return newPrimitiveColumnDefinition(name, parquet.Type_INT32, convertedUint8, logicalUint8)
	case *zed.TypeOfUint16:
		return newPrimitiveColumnDefinition(name, parquet.Type_INT32, convertedUint16, logicalUint16)
	case *zed.TypeOfUint32:
		return newPrimitiveColumnDefinition(name, parquet.Type_INT32, convertedUint32, logicalUint32)
	case *zed.TypeOfUint64:
		return newPrimitiveColumnDefinition(name, parquet.Type_INT64, convertedUint64, logicalUint64)
	case *zed.TypeOfInt8:
		return newPrimitiveColumnDefinition(name, parquet.Type_INT32, convertedInt8, logicalInt8)
	case *zed.TypeOfInt16:
		return newPrimitiveColumnDefinition(name, parquet.Type_INT32, convertedInt16, logicalInt16)
	case *zed.TypeOfInt32:
		return newPrimitiveColumnDefinition(name, parquet.Type_INT32, convertedInt32, logicalInt32)
	case *zed.TypeOfInt64, *zed.TypeOfDuration:
		return newPrimitiveColumnDefinition(name, parquet.Type_INT64, convertedInt64, logicalInt64)
	case *zed.TypeOfTime:
		return newPrimitiveColumnDefinition(name, parquet.Type_INT64, nil, logicalTimestampNanos)
	case *zed.TypeOfFloat16, *zed.TypeOfFloat32:
		return newPrimitiveColumnDefinition(name, parquet.Type_FLOAT, nil, nil)
	case *zed.TypeOfFloat64:
		return newPrimitiveColumnDefinition(name, parquet.Type_DOUBLE, nil, nil)
	// XXX add TypeDecimal
	case *zed.TypeOfBool:
		return newPrimitiveColumnDefinition(name, parquet.Type_BOOLEAN, nil, nil)
	case *zed.TypeOfBytes:
		return newPrimitiveColumnDefinition(name, parquet.Type_BYTE_ARRAY, nil, nil)
	case *zed.TypeOfString, *zed.TypeOfIP, *zed.TypeOfNet, *zed.TypeOfType:
		return newPrimitiveColumnDefinition(name, parquet.Type_BYTE_ARRAY, convertedUTF8, logicalString)
	case *zed.TypeOfNull:
		return nil, ErrNullType
	case *zed.TypeRecord:
		return newRecordColumnDefinition(name, typ)
	case *zed.TypeArray:
		return newListColumnDefinition(name, typ.Type)
	case *zed.TypeSet:
		return newListColumnDefinition(name, typ.Type)
	case *zed.TypeUnion:
		return nil, ErrUnionType
	case *zed.TypeEnum:
		return newPrimitiveColumnDefinition(name, parquet.Type_BYTE_ARRAY, convertedEnum, logicalEnum)
	case *zed.TypeMap:
		return newMapColumnDefinition(name, typ.KeyType, typ.ValType)
	case *zed.TypeError:
		// Errors are formatted as string using their ZSON representation.
		return newPrimitiveColumnDefinition(name, parquet.Type_BYTE_ARRAY, convertedUTF8, logicalString)
	default:
		panic(fmt.Sprintf("unknown type %T", typ))
	}
}

func newPrimitiveColumnDefinition(name string, t parquet.Type, c *parquet.ConvertedType, l *parquet.LogicalType) (*parquetschema.ColumnDefinition, error) {
	return &parquetschema.ColumnDefinition{
		SchemaElement: &parquet.SchemaElement{
			Type:           parquet.TypePtr(t),
			RepetitionType: repetitionOptional,
			Name:           name,
			ConvertedType:  c,
			LogicalType:    l,
		},
	}, nil
}

func newListColumnDefinition(name string, typ zed.Type) (*parquetschema.ColumnDefinition, error) {
	element, err := newColumnDefinition("element", typ)
	if err != nil {
		return nil, err
	}
	return &parquetschema.ColumnDefinition{
		Children: []*parquetschema.ColumnDefinition{
			{
				Children: []*parquetschema.ColumnDefinition{element},
				SchemaElement: &parquet.SchemaElement{
					RepetitionType: repetitionRepeated,
					Name:           "list",
					NumChildren:    int32Ptr(1),
				},
			},
		},
		SchemaElement: &parquet.SchemaElement{
			RepetitionType: repetitionOptional,
			Name:           name,
			NumChildren:    int32Ptr(1),
			ConvertedType:  convertedList,
			LogicalType:    logicalList,
		},
	}, nil
}

func newMapColumnDefinition(name string, keyType, valueType zed.Type) (*parquetschema.ColumnDefinition, error) {
	key, err := newColumnDefinition("key", keyType)
	if err != nil {
		return nil, err
	}
	key.SchemaElement.RepetitionType = repetitionRequired
	value, err := newColumnDefinition("value", valueType)
	if err != nil {
		return nil, err
	}
	value.SchemaElement.RepetitionType = repetitionRequired
	// xxx maybe set key.RepetitionType and value.RepetitionType to repeated
	return &parquetschema.ColumnDefinition{
		Children: []*parquetschema.ColumnDefinition{
			{
				Children: []*parquetschema.ColumnDefinition{key, value},
				SchemaElement: &parquet.SchemaElement{
					RepetitionType: repetitionRepeated,
					Name:           "key_value",
					NumChildren:    int32Ptr(2),
					ConvertedType:  convertedMapKeyValue,
				},
			},
		},
		SchemaElement: &parquet.SchemaElement{
			RepetitionType: repetitionOptional,
			Name:           name,
			NumChildren:    int32Ptr(1),
			ConvertedType:  convertedMap,
			LogicalType:    logicalMap,
		},
	}, nil
}

func newRecordColumnDefinition(name string, typ *zed.TypeRecord) (*parquetschema.ColumnDefinition, error) {
	if len(typ.Fields) == 0 {
		return nil, ErrEmptyRecordType
	}
	var children []*parquetschema.ColumnDefinition
	for _, f := range typ.Fields {
		c, err := newColumnDefinition(f.Name, f.Type)
		if err != nil {
			return nil, err
		}
		children = append(children, c)
	}
	return &parquetschema.ColumnDefinition{
		Children: children,
		SchemaElement: &parquet.SchemaElement{
			RepetitionType: repetitionOptional,
			Name:           name,
			NumChildren:    int32Ptr(len(children)),
		},
	}, nil
}

func int32Ptr(i int) *int32 {
	if i > math.MaxInt32 || i < math.MinInt32 {
		panic(i)
	}
	i32 := int32(i)
	return &i32
}
