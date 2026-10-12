package bigquery

import (
	"encoding/json"
	"sync"
	"testing"

	"github.com/stretchr/testify/suite"
	"gorm.io/gorm/schema"
)

type DataTypeSuite struct {
	suite.Suite
}

func TestDataTypeSuite(t *testing.T) {
	suite.Run(t, new(DataTypeSuite))
}

func (suite *DataTypeSuite) TestDataTypeOf() {
	type TestModel struct {
		JSONField string          `gorm:"column:JSONField;type:JSON"`
		JsonLower json.RawMessage `gorm:"column:JsonLower;type:json"`
		Name      string          `gorm:"column:Name"`
		Count     int             `gorm:"column:Count"`
		Flag      bool            `gorm:"column:Flag"`
		Price     float64         `gorm:"column:Price"`
		Data      []byte          `gorm:"column:Data"`
	}

	s, err := schema.Parse(&TestModel{}, &sync.Map{}, schema.NamingStrategy{})
	suite.NoError(err)
	dialector := Dialector{}

	tests := []struct {
		field    string
		expected string
	}{
		{"JSONField", "JSON"}, // Explicit type:JSON
		{"JsonLower", "json"}, // Explicit type:json (lowercase)
		{"Name", "STRING"},    // String field
		{"Count", "INT64"},    // Int field
		{"Flag", "BOOL"},      // Bool field
		{"Price", "FLOAT64"},  // Float field
		{"Data", "BYTES"},     // Bytes field
	}

	for _, test := range tests {
		suite.Run(test.field, func() {
			field := s.LookUpField(test.field)
			suite.NotNil(field)
			result := dialector.DataTypeOf(field)
			suite.Equal(test.expected, result)
		})
	}
}
