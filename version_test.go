package otelsql_test

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/nhatthm/otelsql"
)

func TestSemVersion(t *testing.T) {
	t.Parallel()

	actual := otelsql.SemVersion()
	expected := fmt.Sprintf("semver:%s", otelsql.Version())

	assert.Equal(t, expected, actual)
}
