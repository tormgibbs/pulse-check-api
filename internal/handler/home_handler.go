package handler

import "net/http"

func HomeHandler(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{
		"name":    "Pulse Check API",
		"version": "1.0.0",
		"message": "Pulse Check API is running",
	})
}