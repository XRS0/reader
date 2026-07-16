package books

import (
	"bytes"
	"errors"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestValidateCover(t *testing.T) {
	png := append([]byte("\x89PNG\r\n\x1a\n"), bytes.Repeat([]byte{0}, 520)...)
	mediaType, extension, err := validateCover(CoverInput{ClientMIME: "image/png", Data: png})
	require.NoError(t, err)
	require.Equal(t, "image/png", mediaType)
	require.Equal(t, ".png", extension)

	_, _, err = validateCover(CoverInput{ClientMIME: "image/jpeg", Data: png})
	require.ErrorIs(t, err, ErrInvalidCover)

	_, _, err = validateCover(CoverInput{Data: []byte("not an image")})
	require.ErrorIs(t, err, ErrInvalidCover)

	_, _, err = validateCover(CoverInput{Data: make([]byte, MaxCoverBytes+1)})
	require.True(t, errors.Is(err, ErrCoverTooLarge))
}
