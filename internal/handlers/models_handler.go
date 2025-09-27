package handlers

import (
	"encoding/json"
	"net/http"

	"github.com/ollama/ollama/api"
)

// ModelsHandler returns a list of available models from the local Ollama server.
func ModelsHandler(apiClient *api.Client) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		resp, err := apiClient.List(ctx)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp.Models)
	})
}
