package bigquery

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"

	"gorm.io/driver/bigquery/driver"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"gorm.io/gorm/logger"
	"gorm.io/gorm/migrator"
	"gorm.io/gorm/schema"
)

type Migrator struct {
	migrator.Migrator
}

func (m Migrator) CurrentDatabase() (name string) {
	datasetID, err := m.getDatasetID()
	if err != nil {
		return ""
	}
	return datasetID
}

func (m Migrator) BuildIndexOptions(opts []schema.IndexOption, stmt *gorm.Statement) (results []interface{}) {
	return
}

func (m Migrator) HasIndex(value interface{}, name string) bool {
	return false
}

func (m Migrator) CreateIndex(value interface{}, name string) error {
	return errors.New("CreateIndex is unsupported")
}

func (m Migrator) RenameIndex(value interface{}, oldName, newName string) error {
	return errors.New("RenameIndex is unsupported")
}

func (m Migrator) DropIndex(value interface{}, name string) error {
	return errors.New("DropIndex is unsupported")
}

func (m Migrator) HasTable(value interface{}) bool {
	var count int64
	m.RunWithValue(value, func(stmt *gorm.Statement) error {
		// According to the BigQuery documentation, an INFORMATION_SCHEMA view must be qualified with a dataset or region.
		// See: https://docs.cloud.google.com/bigquery/docs/information-schema-intro
		//
		// We are going to attempt to get the dataset ID from the connection and use it to qualify the INFORMATION_SCHEMA view.
		datasetID, err := m.getDatasetID()
		if err != nil {
			return err
		}
		return m.DB.Raw("SELECT count(*) FROM `"+datasetID+".INFORMATION_SCHEMA.TABLES` WHERE table_name = ?", stmt.Table).Row().Scan(&count)
	})

	return count > 0
}

func (m Migrator) DropTable(values ...interface{}) error {
	values = m.ReorderModels(values, false)
	tx := m.DB.Session(&gorm.Session{})
	for i := len(values) - 1; i >= 0; i-- {
		if err := m.RunWithValue(values[i], func(stmt *gorm.Statement) error {
			return tx.Exec("DROP TABLE IF EXISTS ?", clause.Table{Name: stmt.Table}).Error
		}); err != nil {
			return err
		}
	}
	return nil
}

func (m Migrator) HasColumn(value interface{}, field string) bool {
	var count int64
	m.RunWithValue(value, func(stmt *gorm.Statement) error {
		name := field
		if field := stmt.Schema.LookUpField(field); field != nil {
			name = field.DBName
		}

		// According to the BigQuery documentation, an INFORMATION_SCHEMA view must be qualified with a dataset or region.
		// See: https://docs.cloud.google.com/bigquery/docs/information-schema-intro
		//
		// We are going to attempt to get the dataset ID from the connection and use it to qualify the INFORMATION_SCHEMA view.
		datasetID, err := m.getDatasetID()
		if err != nil {
			return err
		}

		return m.DB.Raw(
			"SELECT count(*) FROM `"+datasetID+".INFORMATION_SCHEMA.columns` WHERE table_schema = CURRENT_SCHEMA() AND table_name = ? AND column_name = ?",
			stmt.Table, name,
		).Row().Scan(&count)
	})

	return count > 0
}

func (m Migrator) HasConstraint(value interface{}, name string) bool {
	var count int64
	m.RunWithValue(value, func(stmt *gorm.Statement) error {
		// According to the BigQuery documentation, an INFORMATION_SCHEMA view must be qualified with a dataset or region.
		// See: https://docs.cloud.google.com/bigquery/docs/information-schema-intro
		//
		// We are going to attempt to get the dataset ID from the connection and use it to qualify the INFORMATION_SCHEMA view.
		datasetID, err := m.getDatasetID()
		if err != nil {
			return err
		}

		return m.DB.Raw(
			"SELECT count(*) FROM `"+datasetID+".INFORMATION_SCHEMA.table_constraints` WHERE table_schema = CURRENT_SCHEMA() AND table_name = ? AND constraint_name = ?",
			stmt.Table, name,
		).Row().Scan(&count)
	})

	return count > 0
}

// FullDataTypeOf returns field's db full data type
func (m Migrator) FullDataTypeOf(field *schema.Field) (expr clause.Expr) {
	expr.SQL = m.DataTypeOf(field)

	if field.NotNull {
		expr.SQL += " NOT NULL"
	}

	if field.HasDefaultValue && (field.DefaultValueInterface != nil || field.DefaultValue != "") {
		if field.DefaultValueInterface != nil {
			defaultStmt := &gorm.Statement{Vars: []interface{}{field.DefaultValueInterface}}
			m.Dialector.BindVarTo(defaultStmt, defaultStmt, field.DefaultValueInterface)
			expr.SQL += " DEFAULT " + m.Dialector.Explain(defaultStmt.SQL.String(), field.DefaultValueInterface)
		} else if field.DefaultValue != "(-)" {
			expr.SQL += " DEFAULT " + field.DefaultValue
		}
	}

	options := map[string]string{}
	if field.Comment != "" {
		options["description"] = field.Comment
	}

	if len(options) > 0 {
		optionParts := []string{}
		for key, value := range options {
			optionParts = append(optionParts, fmt.Sprintf("%s = %s", key, logger.ExplainSQL("?", nil, `'`, value)))
		}
		slices.Sort(optionParts)
		expr.SQL += " OPTIONS (" + strings.Join(optionParts, " ") + ")"
	}

	return
}

// getDatasetID is a helper function to get the dataset ID from the connection.
func (m Migrator) getDatasetID() (string, error) {
	sqlDB, err := m.DB.DB()
	if err != nil {
		return "", fmt.Errorf("could not get underlying database: %w", err)
	}
	ctx := context.Background()
	conn, err := sqlDB.Conn(ctx)
	if err != nil {
		return "", fmt.Errorf("could not get connection: %w", err)
	}

	datasetID := ""
	err = conn.Raw(func(rawConnection any) error {
		bigQueryConnection, ok := rawConnection.(*driver.BigQueryConnection)
		if !ok {
			return errors.New("connection is not a *driver.BigQueryConnection")
		}
		dataset := bigQueryConnection.GetDataset()
		if dataset == nil {
			return errors.New("dataset is nil")
		}
		datasetID = dataset.DatasetID
		return nil
	})
	if err != nil {
		return "", fmt.Errorf("could not get dataset ID: %w", err)
	}

	return datasetID, nil
}
