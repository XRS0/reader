package auth

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func TestAccessTokenRoundTrip(t *testing.T) {
	m := NewTokenManager("01234567890123456789012345678901", "test", time.Minute, time.Hour)
	uid, did := uuid.New(), uuid.New()
	raw, _, err := m.Access(uid, "person@example.com", did)
	require.NoError(t, err)
	claims, err := m.ParseAccess(raw)
	require.NoError(t, err)
	require.Equal(t, uid.String(), claims.Subject)
	require.Equal(t, did.String(), claims.DeviceID)
}

func TestRefreshTokensHaveDistinctHashes(t *testing.T) {
	m := NewTokenManager("01234567890123456789012345678901", "test", time.Minute, time.Hour)
	a, ah, _, err := m.Refresh()
	require.NoError(t, err)
	b, bh, _, err := m.Refresh()
	require.NoError(t, err)
	require.NotEqual(t, a, b)
	require.NotEqual(t, ah, bh)
	require.Equal(t, HashRefresh(a), ah)
}
