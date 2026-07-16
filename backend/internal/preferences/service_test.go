package preferences

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestWarmThemeValid(t *testing.T) {
	p := Default()
	p.Theme = "warm"
	p.BackgroundColor = "#f5edda"
	p.TextColor = "#3d352a"
	require.NoError(t, Validate(p))
}
func TestInvalidCustomColor(t *testing.T) {
	p := Default()
	p.Theme = "custom"
	p.BackgroundColor = "javascript:alert(1)"
	require.Error(t, Validate(p))
}
