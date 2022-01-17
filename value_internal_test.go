package otelsql

import (
	"database/sql/driver"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestValuesToNamedValues(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		scenario string
		values   []driver.Value
		expected []driver.NamedValue
	}{
		{
			scenario: "nil",
			values:   nil,
			expected: nil,
		},
		{
			scenario: "empty",
			values:   []driver.Value{},
			expected: []driver.NamedValue{},
		},
		{
			scenario: "not empty",
			values:   []driver.Value{"foobar", 42},
			expected: []driver.NamedValue{
				{Ordinal: 1, Value: "foobar"},
				{Ordinal: 2, Value: 42},
			},
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.scenario, func(t *testing.T) {
			t.Parallel()

			actual := valuesToNamedValues(tc.values)

			assert.Equal(t, tc.expected, actual)
		})
	}
}

func TestNamedValuesToValues(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		scenario string
		values   []driver.NamedValue
		expected []driver.Value
	}{
		{
			scenario: "nil",
			values:   nil,
			expected: nil,
		},
		{
			scenario: "empty",
			values:   []driver.NamedValue{},
			expected: []driver.Value{},
		},
		{
			scenario: "not empty",
			values: []driver.NamedValue{
				{Ordinal: 1, Value: "foobar"},
				{Ordinal: 2, Value: 42},
			},
			expected: []driver.Value{"foobar", 42},
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.scenario, func(t *testing.T) {
			t.Parallel()

			actual := namedValuesToValues(tc.values)

			assert.Equal(t, tc.expected, actual)
		})
	}
}
