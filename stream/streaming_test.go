package stream

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewCheckpoint(t *testing.T) {

	testCases := map[string]struct {
		checkpoint string
		shouldErr  bool
		expected   Checkpoint
	}{
		"Given I have a valid checkpoint": {
			checkpoint: "1643296917414-0",
			shouldErr:  false,
			expected:   Checkpoint("1643296917414-0"),
		},
		"Given I have a checkpoint with a timestamp that's too short": {
			checkpoint: "643296917414-0",
			shouldErr:  true,
			expected:   Checkpoint(""),
		},
		"Given I have a checkpoint with no sequence number": {
			checkpoint: "1643296917414-",
			shouldErr:  true,
			expected:   Checkpoint(""),
		},
		"Given I have a checkpoint with no '-'": {
			checkpoint: "16432969174140",
			shouldErr:  true,
			expected:   Checkpoint(""),
		},
		"Given I have the checkpoint 'foobar'": {
			checkpoint: "foobar",
			shouldErr:  true,
			expected:   Checkpoint(""),
		},
	}

	for desc, tc := range testCases {
		tc := tc
		t.Run(desc, func(t *testing.T) {

			t.Log("When I call NewCheckpoint")
			actual, err := NewCheckpoint(tc.checkpoint)
			if (err != nil) != tc.shouldErr {
				t.Errorf("(%s): error = %v, shouldErr = %v", desc, err, tc.shouldErr)
			}

			assert.Equal(t, tc.expected, actual)
		})
	}
}

func TestCheckpoint_IsOlder(t *testing.T) {
	cp1 := Checkpoint("1643021148847-0")
	cp2 := Checkpoint("1643021251176-0")

	seq1 := Checkpoint("1643021148847-1")
	seq2 := Checkpoint("1643021148847-2")

	assert.True(t, cp1.IsOlder(cp2))
	assert.True(t, seq1.IsOlder(seq2))
	assert.True(t, Checkpoint("").IsOlder(cp1))
}

func Test_NewStreamEventFromMap(t *testing.T) {

	testCases := map[string]struct {
		values    map[string]interface{}
		shouldErr bool
		expected  StreamEvent
	}{
		"Given my values map doesn't have an API Key": {
			values: map[string]interface{}{
				StreamEventValueData.String(): "hello world",
			},
			shouldErr: true,
			expected:  StreamEvent{},
		},
		"Given my values map doesn't have a Data key": {
			values: map[string]interface{}{
				StreamEventValueAPIKey.String(): "123",
			},
			shouldErr: true,
			expected:  StreamEvent{},
		},
		"Given my values map has API and Data keys but the API Key value is not a string": {
			values: map[string]interface{}{
				StreamEventValueAPIKey.String(): 123,
				StreamEventValueData.String():   "hello  world",
			},
			shouldErr: true,
			expected:  StreamEvent{},
		},
		"Given my values map has API and Data keys but the Data value is not a string": {
			values: map[string]interface{}{
				StreamEventValueAPIKey.String(): "123",
				StreamEventValueData.String():   858585,
			},
			shouldErr: true,
			expected:  StreamEvent{},
		},
		"Given my values map has API and Data keys and their values are both strings": {
			values: map[string]interface{}{
				StreamEventValueAPIKey.String(): "123",
				StreamEventValueData.String():   "hello world",
			},
			shouldErr: false,
			expected: StreamEvent{
				Values: map[StreamEventValue]string{
					StreamEventValueAPIKey: "123",
					StreamEventValueData:   "hello world",
				},
			},
		},
	}

	for desc, tc := range testCases {
		tc := tc
		t.Run(desc, func(t *testing.T) {

			actual, err := NewStreamEventFromMap(tc.values)
			if (err != nil) != tc.shouldErr {
				t.Errorf("(%s): error = %v, shouldErr = %v", desc, err, tc.shouldErr)
			}

			assert.Equal(t, tc.expected, actual)
		})
	}
}
