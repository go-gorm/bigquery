package bigquery

import (
	"database/sql"
	"database/sql/driver"
	"reflect"
	"strings"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

func initializeBuilders(db *gorm.DB) {
	b := bigQueryBuilders{db}
	db.ClauseBuilders["VALUES"] = b.buildValues
}

type bigQueryBuilders struct {
	db *gorm.DB
}

func (b bigQueryBuilders) buildValues(c clause.Clause, builder clause.Builder) {

	if c.Expression == nil {
		return
	}

	values, ok := c.Expression.(clause.Values)
	if !ok {
		return
	}

	if len(values.Columns) > 0 {
		builder.WriteByte('(')
		for idx, column := range values.Columns {
			if idx > 0 {
				builder.WriteByte(',')
			}
			builder.WriteQuoted(column)
		}
		builder.WriteByte(')')

		builder.WriteString(" VALUES ")

		// precompute which column indexes are JSON-typed, once per statement
		jsonCols := jsonColumnIndexes(builder, values.Columns)

		for idx, value := range values.Values {
			if idx > 0 {
				builder.WriteByte(',')
			}

			builder.WriteByte('(')
			b.buildValuesArguments(builder, value, jsonCols)
			builder.WriteByte(')')
		}
	} else {
		builder.WriteString("DEFAULT VALUES")
	}
}

// jsonColumnIndexes returns a set of column positions whose schema field
// is tagged type:JSON (case-insensitive), so their values get PARSE_JSON wrapping.
func jsonColumnIndexes(builder clause.Builder, columns []clause.Column) map[int]bool {
	result := map[int]bool{}

	stmt, ok := builder.(*gorm.Statement)
	if !ok || stmt.Schema == nil {
		return result
	}

	for idx, column := range columns {
		field := stmt.Schema.LookUpField(column.Name)
		if field == nil {
			continue
		}
		if typ, ok := field.TagSettings["TYPE"]; ok && strings.EqualFold(typ, "JSON") {
			result[idx] = true
		}
	}

	return result
}

func (bigQueryBuilders) buildValuesArguments(builder clause.Builder, vars []interface{}, jsonCols map[int]bool) {
	for idx, v := range vars {
		if idx > 0 {
			builder.WriteByte(',')
		}

		// A field with type:JSON should be wrapped in PARSE_JSON(...) unless it already implements GormValue.
		if jsonCols[idx] && !implementsGormValuer(v) {
			builder.WriteString("PARSE_JSON(")
			writeValue(builder, v)
			builder.WriteByte(')')
			continue
		}

		writeValue(builder, v)
	}
}

// implementsGormValuer reports whether v already knows how to render itself as
// dialect-specific SQL (e.g. datatypes.JSON implementing GormValue)
func implementsGormValuer(v interface{}) bool {
	switch v.(type) {
	case clause.Expr, clause.Expression:
		return true
	}

	_, ok := v.(gorm.Valuer)
	return ok
}

func writeValue(builder clause.Builder, v interface{}) {
	switch v := v.(type) {
	case sql.NamedArg, clause.Column, clause.Table, clause.Expr, driver.Valuer, []byte, []interface{}, *gorm.DB:
		builder.AddVar(builder, v)
	default:
		switch rv := reflect.ValueOf(v); rv.Kind() {
		case reflect.Slice, reflect.Array:
			if rv.Len() == 0 {
				builder.WriteString("[]")
			} else {
				builder.WriteByte('[')
				for i := 0; i < rv.Len(); i++ {
					if i > 0 {
						builder.WriteByte(',')
					}
					builder.AddVar(builder, rv.Index(i).Interface())
				}
				builder.WriteByte(']')
			}
		default:
			builder.AddVar(builder, v)
		}
	}
}
