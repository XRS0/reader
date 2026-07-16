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
