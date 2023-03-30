package token

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func Test_MaskRight(t *testing.T) {
	testCases := map[string]struct {
		input    string
		expected string
	}{
		"Test key gets all characters after first 4 masked": {
			input:    "abcdef12-1234-5678-abcd-012345678901",
			expected: "abcd********************************",
		},
		"Test exactly 4 characters": {
			input:    "abcd",
			expected: "abcd",
		},
		"Test less than 4 characters": {
			input:    "ab",
			expected: "ab",
		},
		"Test empty string": {
			input:    "",
			expected: "",
		},
	}

	for desc, tc := range testCases {
		t.Run(desc, func(t *testing.T) {
			actual := MaskRight(tc.input)
			assert.Equal(t, tc.expected, actual)
		})
	}
}
