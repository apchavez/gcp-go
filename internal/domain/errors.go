package domain

// NotFoundError and ConflictError mirror the AWS TypeScript sibling's shared/errors.ts -
// handlers type-switch on these to map to 404/409 HTTP responses.

type NotFoundError struct {
	Message string
}

func (e *NotFoundError) Error() string { return e.Message }

type ConflictError struct {
	Message string
}

func (e *ConflictError) Error() string { return e.Message }
