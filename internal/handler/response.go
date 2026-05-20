package handler

import (
	"encoding/json"
	"net/http"
)

const (
	codeInternalError     = "INTERNAL_ERROR"
	codeNotFound          = "MONITOR_NOT_FOUND"
	codeDuplicate         = "DUPLICATE_MONITOR"
	codeInvalidBody       = "INVALID_BODY"
	codeInvalidTransition = "INVALID_TRANSITION"
	codeValidationError   = "VALIDATION_ERROR"
)

func writeInternalError(w http.ResponseWriter) {
	writeError(w, http.StatusInternalServerError, "an unexpected error occurred", codeInternalError)
}

func writeNotFound(w http.ResponseWriter) {
	writeError(w, http.StatusNotFound, "monitor not found", codeNotFound)
}

func writeJSON(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if data != nil {
		json.NewEncoder(w).Encode(data)
	}
}

func writeError(w http.ResponseWriter, status int, message, code string) {
	writeJSON(w, status, map[string]string{
		"error": message,
		"code":  code,
	})
}

func writeValidationError(w http.ResponseWriter, fields map[string]string) {
	writeJSON(w, http.StatusUnprocessableEntity, map[string]any{
		"error":  "validation failed",
		"code":   "VALIDATION_ERROR",
		"fields": fields,
	})
}
