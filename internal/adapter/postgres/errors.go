package postgres

import "errors"

// ErrCollectionDocNotFound is returned when a collection document does not
// exist or does not belong to the requested plugin/collection.
var ErrCollectionDocNotFound = errors.New("collection document not found")
