package reading

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestAccountIntervalActive(t *testing.T) {
	start := time.Unix(1000, 0)
	got := AccountInterval(start, start.Add(15*time.Second), true, ActivitySignals{Visible: true, Focused: true, UserActive: true, SinceInteraction: time.Second}, 30*time.Second, time.Minute)
	require.Equal(t, int64(15), got.ActiveSeconds)
	require.Zero(t, got.IdleSeconds)
	require.True(t, got.CurrentActive)
}
func TestAccountIntervalIdleIsNotActive(t *testing.T) {
	start := time.Unix(1000, 0)
	got := AccountInterval(start, start.Add(15*time.Second), true, ActivitySignals{Visible: true, Focused: true, UserActive: true, SinceInteraction: 2 * time.Minute}, 30*time.Second, time.Minute)
	require.Zero(t, got.ActiveSeconds)
	require.Equal(t, int64(15), got.IdleSeconds)
	require.False(t, got.CurrentActive)
}
func TestAccountIntervalCapsAnomalousGap(t *testing.T) {
	start := time.Unix(1000, 0)
	got := AccountInterval(start, start.Add(5*time.Minute), true, ActivitySignals{Visible: true, Focused: true, UserActive: true}, 30*time.Second, time.Minute)
	require.Equal(t, int64(30), got.ActiveSeconds)
	require.Equal(t, int64(270), got.IdleSeconds)
}
func TestAccountIntervalNeedsTwoActiveObservations(t *testing.T) {
	start := time.Unix(1000, 0)
	got := AccountInterval(start, start.Add(15*time.Second), false, ActivitySignals{Visible: true, Focused: true, UserActive: true}, 30*time.Second, time.Minute)
	require.Zero(t, got.ActiveSeconds)
	require.Equal(t, int64(15), got.IdleSeconds)
	require.True(t, got.CurrentActive)
}
