package domain

import (
	"os"
	"testing"
)

// Mocking os.Hostname function
var osHostname = os.Hostname

func Test_getAppID(t *testing.T) {
	// Mock hostname for tests
	mockHostname := "mock-hostname"

	osHostname = func() (string, error) {
		return mockHostname, nil
	}

	tests := []struct {
		name         string
		envAppID     string
		mockHostname string
		mockError    error
		expected     string
	}{
		{
			name:         "APP_ID is set",
			envAppID:     "my-app-id",
			mockHostname: "",
			mockError:    nil,
			expected:     "my-app-id",
		},
		{
			name:         "APP_ID is empty, hostname available",
			envAppID:     "",
			mockHostname: mockHostname,
			mockError:    nil,
			expected:     mockHostname,
		},
		{
			name:         "APP_ID is empty, hostname error",
			envAppID:     "",
			mockHostname: "",
			mockError:    os.ErrNotExist,
			expected:     "unknown",
		},
		{
			name:         "APP_ID is empty, empty hostname",
			envAppID:     "",
			mockHostname: "",
			mockError:    nil,
			expected:     "unknown",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Set environment variable
			os.Setenv("APP_ID", tt.envAppID)

			// Override hostname function to return mock values
			hostnameFunc = func() (string, error) {
				return tt.mockHostname, tt.mockError
			}

			// Execute function
			result := getAppID()

			// Validate result
			if result != tt.expected {
				t.Errorf("Expected %s, got %s", tt.expected, result)
			}
		})
	}
}
