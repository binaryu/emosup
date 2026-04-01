package service

import (
	"errors"
	"strings"

	"emosup/backend/internal/client"
)

type SaveErrorKind string

const (
	SaveErrorKindRetryableWaiting   SaveErrorKind = "retryable_waiting"
	SaveErrorKindRetryableTemporary SaveErrorKind = "retryable_temporary"
	SaveErrorKindFatal              SaveErrorKind = "fatal"
)

func classifySaveError(err error) SaveErrorKind {
	if err == nil {
		return SaveErrorKindFatal
	}

	var saveErr *client.EmosSaveError
	if errors.As(err, &saveErr) {
		if saveErr.RetryableWaiting() {
			return SaveErrorKindRetryableWaiting
		}
		if saveErr.RetryableTemporary() {
			return SaveErrorKindRetryableTemporary
		}
		return SaveErrorKindFatal
	}

	message := strings.ToLower(strings.TrimSpace(err.Error()))
	if strings.Contains(message, "timeout") ||
		strings.Contains(message, "temporary") ||
		strings.Contains(message, "connection reset") ||
		strings.Contains(message, "connection refused") {
		return SaveErrorKindRetryableTemporary
	}

	return SaveErrorKindFatal
}

func isRawURLExpiredMessage(message string) bool {
	normalized := strings.ToLower(strings.TrimSpace(message))
	return strings.Contains(normalized, "403") ||
		strings.Contains(normalized, "404") ||
		strings.Contains(normalized, "signature") ||
		strings.Contains(normalized, "expired") ||
		strings.Contains(normalized, "forbidden") ||
		strings.Contains(normalized, "not found")
}
