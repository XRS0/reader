package books

import (
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func TestStableIDsMakeReprocessingIdempotent(t *testing.T) {
	book := uuid.New()
	a := stableID(book, "chapter:1:0:first")
	b := stableID(book, "chapter:1:0:first")
	c := stableID(book, "chapter:2:0:first")
	require.Equal(t, a, b)
	require.NotEqual(t, a, c)
}
func TestRewriteAssetReferences(t *testing.T) {
	got := rewriteAssetReferences(`<p><img src="../images/cover.jpg"></p>`, "OPS/text/ch1.xhtml", map[string]string{"OPS/images/cover.jpg": "/api/asset/1"})
	require.Contains(t, got, `src="/api/asset/1"`)
}

func TestNormalizeTagsDeduplicatesCaseInsensitively(t *testing.T) {
	tags, err := normalizeTags([]string{" Fiction ", "fiction", "English"})
	require.NoError(t, err)
	require.Equal(t, []string{"Fiction", "English"}, tags)
	_, err = normalizeTags([]string{strings.Repeat("x", 51)})
	require.Error(t, err)
}
