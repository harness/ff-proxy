package domain

import (
	"encoding"
	"errors"
	"fmt"
	"io"
	"net/http"
)

// Stream is a type that writes to an io.Writer
type Stream struct {
	w io.Writer
}

// NewStream creates and returns a Stream with the passed io.Writer
func NewStream(w io.Writer) Stream {
	return Stream{w: w}
}

// Send marshals the event to bytes and writes them to the io.Writer
func (s Stream) Send(event encoding.BinaryMarshaler) error {
	flusher, ok := s.w.(http.Flusher)
	if !ok {
		return errors.New("not a flusher")
	}

	b, err := event.MarshalBinary()
	if err != nil {
		return err
	}

	fmt.Fprintln(s.w, b)
	flusher.Flush()
	return nil
}
