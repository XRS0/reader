package annotations

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestSanitizeNoteBlocks(t *testing.T) {
	raw := json.RawMessage(`[{"type":"text","text":"hello <script>alert(1)</script><b>world</b>"}]`)
	clean, search, err := sanitizeBlocks(raw)
	require.NoError(t, err)
	require.NotContains(t, string(clean), "script")
	require.Contains(t, search, "hello")
	require.Contains(t, search, "world")
}
func TestUnsupportedBlockRejected(t *testing.T) {
	_, _, err := sanitizeBlocks(json.RawMessage(`[{"type":"raw_html","html":"<iframe>"}]`))
	require.Error(t, err)
}

func TestPlainDecodesEntitiesBeforeSanitizing(t *testing.T) {
	require.Equal(t, `"Я есмь"`, plain(`&#34;Я есмь&#34`))
	require.Equal(t, `"Я есмь"`, plain(`&amp;#34;Я есмь&amp;#34;`))
	require.Empty(t, plain(`&lt;script&gt;alert(1)&lt;/script&gt;`))
}
