package domain

// NotFoundError mirrors the AWS TypeScript sibling's shared/errors.ts -
// handlers type-switch on it to map to a 404 HTTP response.

type NotFoundError struct {
	Message string
}

func (e *NotFoundError) Error() string { return e.Message }
