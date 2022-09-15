package otelsql_test

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"

	"go.nhat.io/otelsql"
)

func TestSemVersion(t *testing.T) {
	t.Parallel()

	actual := otelsql.SemVersion()
	expected := fmt.Sprintf("semver:%s", otelsql.Version())

	assert.Equal(t, expected, actual)
}
