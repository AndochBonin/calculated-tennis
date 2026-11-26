package tennisabstract

import "errors"

var (
	// ErrTableNotFound is returned when a required section table cannot be located.
	ErrTableNotFound = errors.New("tennisabstract: table not found")
	// ErrUnexpectedColumns is returned when a table header row does not match expectations.
	ErrUnexpectedColumns = errors.New("tennisabstract: unexpected columns")
	// ErrNoSeasonData is returned when season hold/break cannot be resolved for the requested year.
	ErrNoSeasonData = errors.New("tennisabstract: no season data for evaluation year")
)
