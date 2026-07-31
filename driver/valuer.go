package driver

import (
	"database/sql/driver"
	"fmt"
	"reflect"
)

const maxValuerUnwrapDepth = 100

func unwrapValuer(namedValue *driver.NamedValue) error {
	normalizedValue, err := normalizeDriverValue(namedValue.Value)
	if err != nil {
		return err
	}

	namedValue.Value = normalizedValue
	return nil
}

func normalizeDriverValue(value driver.Value) (driver.Value, error) {
	normalizedValue := value
	for depth := 0; depth < maxValuerUnwrapDepth; depth++ {
		valuer, ok := normalizedValue.(driver.Valuer)
		if !ok {
			return dereferencePointers(normalizedValue), nil
		}

		unwrapped, err := valuer.Value()
		if err != nil {
			return nil, err
		}

		normalizedValue = unwrapped
	}

	return nil, fmt.Errorf("valuer unwrap exceeded max depth %d", maxValuerUnwrapDepth)
}

func dereferencePointers(value driver.Value) driver.Value {
	if value == nil {
		return nil
	}

	v := reflect.ValueOf(value)
	for v.Kind() == reflect.Ptr {
		if v.IsNil() {
			return nil
		}
		v = v.Elem()
	}

	return v.Interface()
}
