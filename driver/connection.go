package driver

import (
	"context"
	"database/sql/driver"
	"fmt"

	"cloud.google.com/go/bigquery"
)

type BigQueryConnection struct {
	ctx     context.Context
	client  *bigquery.Client
	config  bigQueryConfig
	closed  bool
	bad     bool
	dataset *bigquery.Dataset
}

func (connection *BigQueryConnection) GetDataset() *bigquery.Dataset {
	if connection.dataset != nil {
		return connection.dataset
	}
	connection.dataset = connection.client.Dataset(connection.config.dataSet)
	return connection.dataset
}

func (connection *BigQueryConnection) GetContext() context.Context {
	return connection.ctx
}

func (connection *BigQueryConnection) Ping(ctx context.Context) error {

	dataset := connection.GetDataset()
	if dataset == nil {
		return fmt.Errorf("faild to ping using '%s' dataset", connection.config.dataSet)
	}

	_, err := dataset.Metadata(ctx)
	if err != nil {
		return err
	}

	return nil
}

func (connection *BigQueryConnection) QueryContext(ctx context.Context, query string, args []driver.NamedValue) (driver.Rows, error) {
	var statement = &bigQueryStatement{connection, query}
	return statement.QueryContext(ctx, args)
}

func (connection *BigQueryConnection) Query(query string, args []driver.Value) (driver.Rows, error) {
	statement, err := connection.Prepare(query)
	if err != nil {
		return nil, nil
	}

	return statement.Query(args)
}

func (connection *BigQueryConnection) Prepare(query string) (driver.Stmt, error) {
	var statement = &bigQueryStatement{connection, query}

	return statement, nil
}

func (connection *BigQueryConnection) Close() error {
	if connection.closed {
		return nil
	}
	if connection.bad {
		return driver.ErrBadConn
	}
	connection.closed = true
	return connection.client.Close()
}

func (connection *BigQueryConnection) Begin() (driver.Tx, error) {
	var transaction = &bigQueryTransaction{connection}

	return transaction, nil
}

func (connection *BigQueryConnection) query(query string) (*bigquery.Query, error) {
	return connection.client.Query(query), nil
}

func (connection *BigQueryConnection) ExecContext(ctx context.Context, query string, args []driver.NamedValue) (driver.Result, error) {
	var statement = &bigQueryStatement{connection, query}
	return statement.ExecContext(ctx, args)
}

func (connection *BigQueryConnection) Exec(query string, args []driver.Value) (driver.Result, error) {
	var statement = &bigQueryStatement{connection, query}
	return statement.Exec(args)
}

func (connection *BigQueryConnection) CheckNamedValue(namedValue *driver.NamedValue) error {
	return unwrapValuer(namedValue)
}
