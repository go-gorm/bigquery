package bigquery

import (
	"bytes"
	"encoding/json"
	"sync"
	"testing"

	"github.com/stretchr/testify/suite"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"gorm.io/gorm/schema"
)

type BuildersSuite struct {
	suite.Suite
}

func TestBuildersSuite(t *testing.T) {
	suite.Run(t, new(BuildersSuite))
}

// MockBuilder implements clause.Builder for testing
type MockBuilder struct {
	buf  bytes.Buffer
	args []interface{}
	err  error
}

func NewMockBuilder() *MockBuilder {
	return &MockBuilder{
		buf:  bytes.Buffer{},
		args: []interface{}{},
	}
}

func (mb *MockBuilder) WriteByte(b byte) error {
	return mb.buf.WriteByte(b)
}

func (mb *MockBuilder) WriteString(s string) (int, error) {
	return mb.buf.WriteString(s)
}

func (mb *MockBuilder) WriteQuoted(field interface{}) {
	switch f := field.(type) {
	case clause.Column:
		mb.buf.WriteByte('`')
		mb.buf.WriteString(f.Name)
		mb.buf.WriteByte('`')
	case string:
		mb.buf.WriteByte('`')
		mb.buf.WriteString(f)
		mb.buf.WriteByte('`')
	}
}

func (mb *MockBuilder) AddVar(_ clause.Writer, values ...interface{}) {
	mb.buf.WriteString("?")
	mb.args = append(mb.args, values...)
}

func (mb *MockBuilder) AddError(err error) error {
	if mb.err == nil {
		mb.err = err
	}
	return mb.err
}

func (mb *MockBuilder) String() string {
	return mb.buf.String()
}

func (mb *MockBuilder) GetArgs() []interface{} {
	return mb.args
}

// Test PARSE_JSON wrapping in buildValuesArguments
func (suite *BuildersSuite) TestBuildValuesArguments() {
	tests := []struct {
		name     string
		vars     []interface{}
		jsonCols map[int]bool
		expected string
	}{
		{
			name:     "no json columns",
			vars:     []interface{}{"v1", "v2"},
			jsonCols: map[int]bool{},
			expected: "?,?",
		},
		{
			name:     "first column is json",
			vars:     []interface{}{"v1", "v2"},
			jsonCols: map[int]bool{0: true},
			expected: "PARSE_JSON(?),?",
		},
		{
			name:     "second column is json",
			vars:     []interface{}{"v1", "v2"},
			jsonCols: map[int]bool{1: true},
			expected: "?,PARSE_JSON(?)",
		},
		{
			name:     "multiple json columns",
			vars:     []interface{}{"v1", "v2", "v3"},
			jsonCols: map[int]bool{0: true, 2: true},
			expected: "PARSE_JSON(?),?,PARSE_JSON(?)",
		},
	}

	for _, tt := range tests {
		suite.Run(tt.name, func() {
			builder := NewMockBuilder()
			b := bigQueryBuilders{}
			b.buildValuesArguments(builder, tt.vars, tt.jsonCols)
			suite.Equal(tt.expected, builder.String())
		})
	}
}

// Test JSON column detection
func (suite *BuildersSuite) TestJsonColumnIndexes() {
	type TestModel struct {
		Name     string          `gorm:"column:Name"`
		JSONData json.RawMessage `gorm:"column:JSONData;type:JSON"`
		Metadata string          `gorm:"column:Metadata;type:json"` // lowercase
	}

	s, err := schema.Parse(&TestModel{}, &sync.Map{}, schema.NamingStrategy{})
	suite.NoError(err)

	stmt := &gorm.Statement{Schema: s}
	columns := []clause.Column{
		{Name: "Name"},
		{Name: "JSONData"},
		{Name: "Metadata"},
	}

	result := jsonColumnIndexes(stmt, columns)

	// JSONData and Metadata should be marked as JSON columns
	suite.True(result[1], "JSONData at index 1 should be JSON")
	suite.True(result[2], "Metadata at index 2 should be JSON")
	suite.False(result[0], "Name at index 0 should not be JSON")
}

// Test buildValues generates correct SQL
func (suite *BuildersSuite) TestBuildValues() {
	suite.Run("with columns", func() {
		builder := NewMockBuilder()
		b := bigQueryBuilders{}
		clauseExpr := clause.Clause{
			Expression: clause.Values{
				Columns: []clause.Column{
					{Name: "Name"},
					{Name: "Age"},
				},
				Values: [][]interface{}{
					{"John", 30},
				},
			},
		}

		b.buildValues(clauseExpr, builder)
		result := builder.String()

		suite.Contains(result, "(`Name`,`Age`)")
		suite.Contains(result, "VALUES")
	})

	suite.Run("no columns returns default", func() {
		builder := NewMockBuilder()
		b := bigQueryBuilders{}
		clauseExpr := clause.Clause{
			Expression: clause.Values{
				Columns: []clause.Column{},
			},
		}

		b.buildValues(clauseExpr, builder)
		suite.Equal("DEFAULT VALUES", builder.String())
	})
}
