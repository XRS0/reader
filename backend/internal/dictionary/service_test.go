package dictionary

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestNormalizeWordForDeduplication(t *testing.T) {
	require.Equal(t, "hello world", NormalizeWord("  HELLO\t World  "))
	require.Equal(t, NormalizeWord("Book"), NormalizeWord(" book "))
}
func TestDictionaryStatuses(t *testing.T) {
	for _, status := range []string{"unknown", "learning", "known", "mastered", "ignored"} {
		require.True(t, validStatus(status))
	}
	require.False(t, validStatus("super-known"))
}
