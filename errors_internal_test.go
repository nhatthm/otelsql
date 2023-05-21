package otelsql

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestHandleError(t *testing.T) {
	t.Parallel()

	assert.Panics(t, func() {
		mustNoError(errors.New("error"))
	})

	assert.NotPanics(t, func() {
		mustNoError(nil)
	})

	assert.NotPanics(t, func() {
		handleErr(nil)
	})

	assert.NotPanics(t, func() {
		handleErr(assert.AnError)
	})
}
