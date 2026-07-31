package driver

import (
	"database/sql/driver"
	"errors"
	"testing"

	"github.com/stretchr/testify/require"
)

type staticValuer struct {
	value driver.Value
}

func (valuer staticValuer) Value() (driver.Value, error) {
	return valuer.value, nil
}

type errorValuer struct {
	err error
}

func (valuer errorValuer) Value() (driver.Value, error) {
	return nil, valuer.err
}

type recursiveValuer struct{}

func (recursiveValuer) Value() (driver.Value, error) {
	return recursiveValuer{}, nil
}

func TestBigQueryConnectionCheckNamedValueUnwrapsNestedValuer(t *testing.T) {
	namedValue := &driver.NamedValue{
		Name:  "value",
		Value: staticValuer{value: staticValuer{value: "hello"}},
	}

	err := (&bigQueryConnection{}).CheckNamedValue(namedValue)

	require.NoError(t, err)
	require.Equal(t, "hello", namedValue.Value)
}

func TestBigQueryStatementCheckNamedValuePropagatesValuerError(t *testing.T) {
	expectedErr := errors.New("fail to unwrap")
	namedValue := &driver.NamedValue{
		Value: errorValuer{err: expectedErr},
	}

	err := bigQueryStatement{}.CheckNamedValue(namedValue)

	require.ErrorIs(t, err, expectedErr)
}

func TestBigQueryStatementCheckNamedValueGuardsAgainstInfiniteUnwrap(t *testing.T) {
	namedValue := &driver.NamedValue{
		Value: recursiveValuer{},
	}

	err := bigQueryStatement{}.CheckNamedValue(namedValue)

	require.EqualError(t, err, "valuer unwrap exceeded max depth 100")
}

func TestBigQueryStatementCheckNamedValueUnwrapsPointers(t *testing.T) {
	value := true
	namedValue := &driver.NamedValue{Value: &value}

	err := bigQueryStatement{}.CheckNamedValue(namedValue)

	require.NoError(t, err)
	require.Equal(t, true, namedValue.Value)
}

func TestBigQueryStatementCheckNamedValueConvertsNilPointersToNil(t *testing.T) {
	var value *bool
	namedValue := &driver.NamedValue{Value: value}

	err := bigQueryStatement{}.CheckNamedValue(namedValue)

	require.NoError(t, err)
	require.Nil(t, namedValue.Value)
}

func TestBuildParameterFromNamedValueUnwrapsPointers(t *testing.T) {
	value := true
	namedValue := driver.NamedValue{Name: "flag", Value: &value}

	params := buildParameterFromNamedValue(namedValue, nil)

	require.Len(t, params, 1)
	require.Equal(t, "flag", params[0].Name)
	require.Equal(t, true, params[0].Value)
}
