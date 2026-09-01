package driver

import (
	"context"
	"database/sql/driver"
	"errors"

	"cloud.google.com/go/bigquery"
	"github.com/sirupsen/logrus"
	"gorm.io/driver/bigquery/adaptor"
)

type bigQueryStatement struct {
	connection *BigQueryConnection
	query      string
}

func (statement bigQueryStatement) Close() error {
	return nil
}

func (statement bigQueryStatement) NumInput() int {
	return 0
}

func (bigQueryStatement) CheckNamedValue(namedValue *driver.NamedValue) error {
	return unwrapValuer(namedValue)
}

func (statement *bigQueryStatement) ExecContext(ctx context.Context, args []driver.NamedValue) (driver.Result, error) {
	logrus.Debugf("exec:%s", statement.query)

	if logrus.IsLevelEnabled(logrus.DebugLevel) {
		for _, arg := range args {
			logrus.Debugf("- param:%s", convertParameterToValue(arg))
		}
	}

	query, err := statement.buildQuery(convertParameters(args))
	if err != nil {
		return nil, err
	}

	rowIterator, err := query.Read(ctx)
	if err != nil {
		return nil, err
	}

	return &bigQueryResult{rowIterator}, nil
}

func (statement *bigQueryStatement) QueryContext(ctx context.Context, args []driver.NamedValue) (driver.Rows, error) {

	logrus.Debugf("query:%s", statement.query)

	if logrus.IsLevelEnabled(logrus.DebugLevel) {
		for _, arg := range args {
			logrus.Debugf("- param:%s", convertParameterToValue(arg))
		}
	}

	if statement.query == adaptor.RerouteQuery {

		if len(args) < 1 {
			return nil, errors.New("expected a rerouting argument")
		}

		column, ok := args[0].Value.(bigQueryReroutedColumn)
		if !ok {
			return nil, errors.New("expected a rerouting argument with rows")
		}

		schemaAdaptor := adaptor.GetSchemaAdaptor(ctx)
		if schemaAdaptor == nil {
			return nil, errors.New("expected a rerouting schema adaptor")
		}

		schema := createBigQuerySchema(column.schema, schemaAdaptor)

		return &bigQueryRows{
			source: createSourceFromColumn(schema, column.values),
		}, nil
	}

	query, err := statement.buildQuery(convertParameters(args))
	if err != nil {
		return nil, err
	}

	rowIterator, err := query.Read(context.Background())
	if err != nil {
		return nil, err
	}

	return &bigQueryRows{
		source: createSourceFromRowIterator(rowIterator, adaptor.GetSchemaAdaptor(ctx)),
	}, nil

}

func (statement bigQueryStatement) Exec(args []driver.Value) (driver.Result, error) {

	logrus.Debugf("exec:%s", statement.query)

	if logrus.IsLevelEnabled(logrus.DebugLevel) {
		for _, arg := range args {
			logrus.Debugf("- param:%s", convertParameterToValue(arg))
		}
	}

	query, err := statement.buildQuery(args)
	if err != nil {
		return nil, err
	}

	rowIterator, err := query.Read(context.Background())
	if err != nil {
		return nil, err
	}

	return &bigQueryResult{rowIterator}, nil
}

func (statement bigQueryStatement) Query(args []driver.Value) (driver.Rows, error) {

	logrus.Debugf("query:%s", statement.query)
	if logrus.IsLevelEnabled(logrus.DebugLevel) {
		for _, arg := range args {
			logrus.Debugf("- param:%s", convertParameterToValue(arg))
		}
	}

	query, err := statement.buildQuery(args)
	if err != nil {
		return nil, err
	}

	rowIterator, err := query.Read(context.Background())
	if err != nil {
		return nil, err
	}

	return &bigQueryRows{source: createSourceFromRowIterator(rowIterator, nil)}, nil
}

func (statement bigQueryStatement) buildQuery(args []driver.Value) (*bigquery.Query, error) {

	query, err := statement.connection.query(statement.query)
	if err != nil {
		return nil, err
	}
	query.DefaultDatasetID = statement.connection.config.dataSet
	query.Parameters, err = statement.buildParameters(args)
	if err != nil {
		return nil, err
	}

	return query, err
}

func (statement bigQueryStatement) buildParameters(args []driver.Value) ([]bigquery.QueryParameter, error) {
	if args == nil {
		return nil, nil
	}

	var parameters []bigquery.QueryParameter
	for _, arg := range args {
		parameters = buildParameter(arg, parameters)
	}
	return parameters, nil
}

func buildParameter(arg driver.Value, parameters []bigquery.QueryParameter) []bigquery.QueryParameter {
	namedValue, ok := arg.(driver.NamedValue)
	if ok {
		return buildParameterFromNamedValue(namedValue, parameters)
	}

	normalizedValue := dereferencePointers(arg)
	logrus.Debugf("-param:%s", normalizedValue)

	return append(parameters, bigquery.QueryParameter{
		Value: normalizedValue,
	})
}

func buildParameterFromNamedValue(namedValue driver.NamedValue, parameters []bigquery.QueryParameter) []bigquery.QueryParameter {
	normalizedValue := dereferencePointers(namedValue.Value)
	logrus.Debugf("-param:%s=%s", namedValue.Name, normalizedValue)

	if namedValue.Name == "" {
		return append(parameters, bigquery.QueryParameter{
			Value: normalizedValue,
		})
	} else {
		return append(parameters, bigquery.QueryParameter{
			Name:  namedValue.Name,
			Value: normalizedValue,
		})
	}
}

func convertParameters(args []driver.NamedValue) []driver.Value {
	var values []driver.Value
	if args != nil {
		for _, arg := range args {
			values = append(values, arg)
		}
	}
	return values
}
func convertParameterToValue(value driver.Value) interface{} {
	namedValue, ok := value.(driver.NamedValue)
	if ok {
		return namedValue.Value
	}
	return value
}
