package otelsql_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"

	"go.nhat.io/otelsql"
)

func TestQueryContext(t *testing.T) {
	t.Parallel()

	actual := otelsql.QueryFromContext(context.Background())
	assert.Empty(t, actual)

	ctx := otelsql.ContextWithQuery(context.Background(), "SELECT 1")
	actual = otelsql.QueryFromContext(ctx)
	expected := "SELECT 1"

	assert.Equal(t, expected, actual)
}
