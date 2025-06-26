package attribute_test

import (
	"database/sql/driver"
	"fmt"
	"math/rand"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/otel/attribute"

	xattr "go.nhat.io/otelsql/attribute"
)

const key = attribute.Key("key")

func BenchmarkFromNamedValue(b *testing.B) {
	const pattern = `xMROXgoHsB8Y5yTH`

	longString := strings.Repeat(pattern, 17)

	data := []any{
		nil,
		10,
		int64(42),
		3.14,
		true,
		[]byte(longString),
		[]byte(`foobar`),
		longString,
		`foobar`,
		10,
		[]int64{42},
		[]float64{3.14},
		[]bool{true},
		ptrOf(10),
		ptrOf(int64(42)),
		ptrOf(3.14),
		ptrOf(true),
		ptrOf(longString),
		ptrOf(`foobar`),
		map[string]string{"hello": "world"},
	}

	namedValues := make([]driver.NamedValue, len(data))

	for i, v := range data {
		namedValues[i] = driver.NamedValue{
			Ordinal: i + 1,
			Value:   v,
		}
	}

	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		for _, v := range namedValues {
			xattr.FromNamedValue(v)
		}
	}
}

func BenchmarkKeyValueDuration(b *testing.B) {
	r := rand.New(rand.NewSource(time.Now().UnixNano())) // nolint: gosec
	d := time.Duration(r.Int63n(int64(10 * time.Second)))

	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		xattr.KeyValueDuration(key, d)
	}
}

func TestFromNamedValue(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		scenario string
		value    driver.NamedValue
		expected attribute.KeyValue
	}{
		{
			scenario: "with name",
			value: driver.NamedValue{
				Name:  "country",
				Value: `US`,
			},
			expected: attribute.String("db.sql.args.country", `US`),
		},
		{
			scenario: "with name",
			value: driver.NamedValue{
				Ordinal: 42,
				Value:   `US`,
			},
			expected: attribute.String("db.sql.args.42", `US`),
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.scenario, func(t *testing.T) {
			t.Parallel()

			actual := xattr.FromNamedValue(tc.value)

			assert.Equal(t, tc.expected, actual)
		})
	}
}

func TestKeyValue(t *testing.T) {
	t.Parallel()

	const pattern = `xMROXgoHsB8Y5yTH`

	longString := strings.Repeat(pattern, 17)
	shortenedString := fmt.Sprintf("%s... (more than 256 chars)", longString[0:231])

	testCases := []struct {
		scenario string
		value    any
		expected attribute.KeyValue
	}{
		{
			scenario: "nil",
			expected: key.String(""),
		},
		{
			scenario: "int",
			value:    10,
			expected: key.Int(10),
		},
		{
			scenario: "int64",
			value:    int64(42),
			expected: key.Int64(42),
		},
		{
			scenario: "float64",
			value:    3.14,
			expected: key.Float64(3.14),
		},
		{
			scenario: "bool",
			value:    true,
			expected: key.Bool(true),
		},
		{
			scenario: "long []byte",
			value:    []byte(longString),
			expected: key.String(shortenedString),
		},
		{
			scenario: "[]byte",
			value:    []byte(`foobar`),
			expected: key.String(`foobar`),
		},
		{
			scenario: "long string",
			value:    longString,
			expected: key.String(shortenedString),
		},
		{
			scenario: "string",
			value:    `foobar`,
			expected: key.String(`foobar`),
		},
		{
			scenario: "[]int",
			value:    []int{10},
			expected: key.IntSlice([]int{10}),
		},
		{
			scenario: "[]int64",
			value:    []int64{42},
			expected: key.Int64Slice([]int64{42}),
		},
		{
			scenario: "[]float64",
			value:    []float64{3.14},
			expected: key.Float64Slice([]float64{3.14}),
		},
		{
			scenario: "[]bool",
			value:    []bool{true},
			expected: key.BoolSlice([]bool{true}),
		},
		{
			scenario: "*int",
			value:    ptrOf(10),
			expected: key.Int(10),
		},
		{
			scenario: "*int64",
			value:    ptrOf(int64(42)),
			expected: key.Int64(42),
		},
		{
			scenario: "*float64",
			value:    ptrOf(3.14),
			expected: key.Float64(3.14),
		},
		{
			scenario: "*bool",
			value:    ptrOf(true),
			expected: key.Bool(true),
		},
		{
			scenario: "long *string",
			value:    ptrOf(longString),
			expected: key.String(shortenedString),
		},
		{
			scenario: "*string",
			value:    ptrOf(`foobar`),
			expected: key.String(`foobar`),
		},
		{
			scenario: "nil *int",
			value:    (*int)(nil),
			expected: key.String(""),
		},
		{
			scenario: "nil *int64",
			value:    (*int64)(nil),
			expected: key.String(""),
		},
		{
			scenario: "nil *float64",
			value:    (*float64)(nil),
			expected: key.String(""),
		},
		{
			scenario: "nil *bool",
			value:    (*bool)(nil),
			expected: key.String(""),
		},
		{
			scenario: "nil *string",
			value:    (*string)(nil),
			expected: key.String(""),
		},
		{
			scenario: "duration",
			value:    time.Second,
			expected: key.String(`1s`),
		},
		{
			scenario: "other",
			value:    map[string]string{"hello": "world"},
			expected: key.String(`map[hello:world]`),
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.scenario, func(t *testing.T) {
			t.Parallel()

			actual := xattr.KeyValue(key, tc.value)

			assert.Equal(t, tc.expected, actual)
		})
	}
}

func TestKeyValueDuration(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		scenario string
		duration time.Duration
		expected attribute.KeyValue
	}{
		{
			scenario: "0s",
			expected: key.String("0s"),
		},
		{
			scenario: "1s",
			duration: time.Second,
			expected: key.String("1s"),
		},
		{
			scenario: "more than 1 second",
			duration: 10*time.Second + time.Minute,
			expected: key.String("1m10s"),
		},
		{
			scenario: "nanoseconds",
			duration: 116 * time.Nanosecond,
			expected: key.String("116ns"),
		},
		{
			scenario: "microseconds",
			duration: 110 * time.Microsecond,
			expected: key.String("110us"),
		},
		{
			scenario: "milliseconds",
			duration: 117 * time.Millisecond,
			expected: key.String("117ms"),
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.scenario, func(t *testing.T) {
			t.Parallel()

			actual := xattr.KeyValueDuration(key, tc.duration)

			assert.Equal(t, tc.expected, actual)
		})
	}
}

func ptrOf(v any) any {
	val := reflect.ValueOf(v)
	p := reflect.New(val.Type())

	p.Elem().Set(val)

	return p.Interface()
}
