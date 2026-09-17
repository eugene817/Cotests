// Package judge defines the placeholder boundary for future judging.
// The engine and its result protocol will be designed with the professor.
package judge

import (
	"context"
	"errors"
)

// ErrUnavailable means no evaluation was requested or performed.
var ErrUnavailable = errors.New("judge is not available yet")

// Judge is a provisional boundary for requesting evaluation of a saved submission.
// It deliberately specifies no runner, transport, result schema or scoring policy.
type Judge interface {
	RequestEvaluation(ctx context.Context, submissionID uint) error
}

// Noop is the placeholder judge. It has no dependencies or side effects.
// Callers must display ErrUnavailable as unjudged, never queued or accepted.
type Noop struct{}

var _ Judge = Noop{}

func (Noop) RequestEvaluation(context.Context, uint) error {
	return ErrUnavailable
}
