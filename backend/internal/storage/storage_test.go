package storage

import (
	"context"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestMemoryStoreAndLocationValidation(t *testing.T) {
	s := NewMemoryStore()
	require.NoError(t, s.Put(context.Background(), "books-original", "users/u/books/b/original/hash", strings.NewReader("book"), 4, "text/plain", nil))
	data, err := s.Get(context.Background(), "books-original", "users/u/books/b/original/hash", 10)
	require.NoError(t, err)
	require.Equal(t, "book", string(data))
	require.False(t, validObject("books-original", "../secret"))
	require.False(t, validObject("books-original", "/absolute"))
}
