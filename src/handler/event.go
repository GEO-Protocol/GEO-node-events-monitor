package handler

import (
	"errors"
	"strconv"
	"strings"
)

type Event struct {
	Code   int
	Tokens []string
	Error  error
}

// Parses the event from raw bytes sequence
// (often from the events.fifo file of some node)
//
// Returns result even if bytes sequence was not parsed correctly.
// In that case, the Error field of the result would be set.
func EventFromRawInput(body []byte) *Event {
	contentWithoutTrailingSymbol := body[:len(body)-1]
	tokens := strings.Split(string(contentWithoutTrailingSymbol), string('\t'))
	code, err := strconv.Atoi(tokens[0])
	if err != nil {
		return &Event{
			Error: errors.New("Can't parse event code."),
		}
	}

	return &Event{
		Code:   code,
		Tokens: tokens[1:],
	}
}

