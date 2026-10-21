package db

import "errors"

var ErrNotDraft = errors.New("only a draft revision can be edited")

type ValidationError struct{ Message string }

func (e ValidationError) Error() string { return e.Message }
