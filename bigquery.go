package bigquery

import (
	"database/sql"
	"fmt"
	"gorm.io/driver/bigquery/adaptor"
	_ "gorm.io/driver/bigquery/driver"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"gorm.io/gorm/logger"
	"gorm.io/gorm/migrator"
	"gorm.io/gorm/schema"
	"reflect"
	"regexp"
	"strings"
)

type Dialector struct {
	*Config
}

type Config struct {
	DSN                  string
	PreferSimpleProtocol bool
	Conn                 *sql.DB
}

func Open(dsn string) gorm.Dialector {
	return &Dialector{&Config{DSN: dsn}}
}

func (Dialector) Name() string {
	return "bigquery"
}

func (dialector Dialector) Initialize(db *gorm.DB) (err error) {

	initializeCallbacks(db)

	initializeBuilders(db)

	if dialector.Conn != nil {
		db.ConnPool = dialector.Conn
	} else {
		db.ConnPool, err = sql.Open("bigquery", dialector.Config.DSN)
	}

	return
}

func (dialector Dialector) Migrator(db *gorm.DB) gorm.Migrator {
	return Migrator{migrator.Migrator{Config: migrator.Config{
		DB:                          db,
		Dialector:                   dialector,
		CreateIndexAfterCreateTable: false,
	}}}
}

func (Dialector) DefaultValueOf(field *schema.Field) clause.Expression {
	return clause.Expr{SQL: "DEFAULT"}
}

func (Dialector) BindVarTo(writer clause.Writer, stmt *gorm.Statement, v interface{}) {
	writer.WriteByte('?')
}

func (Dialector) QuoteTo(writer clause.Writer, str string) {
	writer.WriteByte('`')
	writer.WriteString(str)
	writer.WriteByte('`')
}

var numericPlaceholder = regexp.MustCompile("\\$(\\d+)")

func (Dialector) Explain(sql string, vars ...interface{}) string {
	return logger.ExplainSQL(sql, numericPlaceholder, `'`, vars...)
}

func (dialector Dialector) DataTypeOf(field *schema.Field) string {
	switch field.DataType {
	case schema.Bool:
		return "BOOL"
	case schema.Int, schema.Uint:
		return "INT64"
	case schema.Float:
		return "FLOAT64"
	case schema.String:
		return "STRING"
	case schema.Time:
		// Distinguish between schema.Time and tag time
		if val, ok := field.TagSettings["TYPE"]; ok {
			return val
		} else {
			return "TIMESTAMP"
		}
	case schema.Bytes:
		return "BYTES"
	}

	switch field.DataType {
	case adaptor.RecordType:
		return dialector.dataTypeOfNested("STRUCT<%s>", field)
	case adaptor.ArrayType:
		return dialector.dataTypeOfNested("ARRAY<STRUCT<%s>>", field)
	}
	return string(field.DataType)
}

func (dialector Dialector) dataTypeOfNested(format string, field *schema.Field) string {

	var structFields []*schema.Field

	fieldType := field.FieldType

	if fieldType.Kind() == reflect.Slice || fieldType.Kind() == reflect.Array {
		fieldType = fieldType.Elem()
	}

	for index := 0; index < fieldType.NumField(); index++ {
		structFields = append(structFields, field.Schema.ParseField(fieldType.Field(index)))
	}

	var fieldDefinitions []string
	for _, structField := range structFields {
		fieldDefinitions = append(fieldDefinitions, fmt.Sprintf("%s %s", structField.Name, dialector.DataTypeOf(structField)))
	}

	return fmt.Sprintf(format, strings.Join(fieldDefinitions, ", "))
}

func (Dialector) SavePoint(tx *gorm.DB, name string) error {
	tx.Exec("SAVEPOINT " + name)
	return nil
}

func (Dialector) RollbackTo(tx *gorm.DB, name string) error {
	tx.Exec("ROLLBACK TO SAVEPOINT " + name)
	return nil
}
