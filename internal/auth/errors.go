package auth

import "errors"

var (
	// ErrMissingIdentity means the request omitted client or project, which
	// would create an unattributable key.
	ErrMissingIdentity = errors.New("client and project are required")
	// ErrNegativeLimit rejects quotas that can never be satisfied.
	ErrNegativeLimit = errors.New("limits must be non-negative")
	// ErrNotFound reports an unknown or ambiguous key identifier.
	ErrNotFound = errors.New("client key not found")
)
