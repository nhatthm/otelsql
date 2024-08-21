package assert

import (
	"github.com/stretchr/testify/assert"
	"github.com/swaggest/assertjson"
)

// Func asserts actual input.
type Func func(t assert.TestingT, actual string, msgAndArgs ...any) bool

// Equal creates a new Func to check whether the two values are equal.
func Equal(expect string) Func {
	return func(t assert.TestingT, actual string, msgAndArgs ...any) bool {
		return assert.Equal(t, expect, actual, msgAndArgs...)
	}
}

// EqualJSON creates a new Func to check whether the two JSON values are equal.
func EqualJSON(expect string) Func {
	return func(t assert.TestingT, actual string, msgAndArgs ...any) bool {
		return assertjson.Equal(t, []byte(expect), []byte(actual), msgAndArgs...)
	}
}

// Nop creates a new Func that does not assert anything.
func Nop() Func {
	return func(_ assert.TestingT, _ string, _ ...any) bool {
		return true
	}
}

// Empty creates a new Func to check whether the actual data is empty.
func Empty() Func {
	return Equal("")
}
