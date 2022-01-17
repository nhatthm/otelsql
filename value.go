package otelsql

import "database/sql/driver"

func valuesToNamedValues(values []driver.Value) []driver.NamedValue {
	if values == nil {
		return nil
	}

	namedValues := make([]driver.NamedValue, len(values))

	for i, v := range values {
		namedValues[i] = driver.NamedValue{
			Ordinal: i + 1,
			Value:   v,
		}
	}

	return namedValues
}

func namedValuesToValues(namedValues []driver.NamedValue) []driver.Value {
	if namedValues == nil {
		return nil
	}

	values := make([]driver.Value, len(namedValues))

	for i, v := range namedValues {
		values[i] = v.Value
	}

	return values
}
