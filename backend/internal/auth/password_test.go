package auth

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestPasswordHasher(t *testing.T) {
	h := NewPasswordHasher(8*1024, 1, 1)
	encoded, err := h.Hash("correct horse battery staple")
	require.NoError(t, err)
	ok, err := h.Verify(encoded, "correct horse battery staple")
	require.NoError(t, err)
	require.True(t, ok)
	ok, err = h.Verify(encoded, "incorrect password")
	require.NoError(t, err)
	require.False(t, ok)
}

func TestPasswordRejectsUnsafeEncodedParameters(t *testing.T) {
	h := NewPasswordHasher(8*1024, 1, 1)
	_, err := h.Verify("$argon2id$v=19$m=4294967295,t=3,p=2$c2FsdHNhbHQ$YWJjZGVmZ2hpamtsbW5vcA", "password")
	require.Error(t, err)
}
