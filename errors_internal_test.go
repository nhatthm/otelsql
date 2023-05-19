package otelsql

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestHandleError(t *testing.T) {
	t.Parallel()

	assert.Panics(t, func() {
		mustHandleErr(errors.New("error"))
	})

	assert.NotPanics(t, func() {
		mustHandleErr(nil)
	})

	assert.NotPanics(t, func() {
		handleErr(nil)
	})

	assert.NotPanics(t, func() {
		handleErr(assert.AnError)
	})
}
