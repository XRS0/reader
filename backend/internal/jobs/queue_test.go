package jobs

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestRetryDelayIsExponentialAndCapped(t *testing.T) {
	require.Equal(t, time.Second, RetryDelay(1))
	require.Equal(t, 2*time.Second, RetryDelay(2))
	require.Equal(t, 4*time.Second, RetryDelay(3))
	require.Equal(t, 5*time.Minute, RetryDelay(100))
}
