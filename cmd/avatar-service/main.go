package main

import (
	"encoding/json"
	"errors"
	"io"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/infrai-examples/health-avatar-pipeline/internal/avatars"
	"github.com/infrai-examples/health-avatar-pipeline/internal/infrai"
)

const maxAvatarBytes = 8 << 20

func main() {
	key := os.Getenv("INFRAI_API_KEY")
	if key == "" {
		log.Fatal("INFRAI_API_KEY is required")
	}
	baseURL := os.Getenv("INFRAI_BASE_URL")
	if baseURL == "" {
		baseURL = infrai.DefaultBaseURL
	}

	pipeline := avatars.New(infrai.New(baseURL, key, nil))
	mux := http.NewServeMux()
	mux.HandleFunc("POST /avatars/{user_id}", uploadHandler(pipeline))
	server := &http.Server{Addr: ":8080", Handler: mux, ReadHeaderTimeout: 5 * time.Second}
	log.Printf("avatar service listening on %s", server.Addr)
	log.Fatal(server.ListenAndServe())
}

func uploadHandler(pipeline *avatars.Pipeline) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		r.Body = http.MaxBytesReader(w, r.Body, maxAvatarBytes)
		if err := r.ParseMultipartForm(maxAvatarBytes); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid multipart form"})
			return
		}
		file, header, err := r.FormFile("avatar")
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "avatar is required"})
			return
		}
		defer file.Close()
		contents, err := io.ReadAll(file)
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "could not read avatar"})
			return
		}

		requestID := r.Header.Get("Idempotency-Key")
		if requestID == "" {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "Idempotency-Key is required"})
			return
		}
		result, err := pipeline.Run(r.Context(), avatars.Input{
			UserID: r.PathValue("user_id"), Filename: header.Filename, File: contents,
			RequestID: requestID, AppointmentStatus: avatars.AppointmentStatus(r.FormValue("appointment_status")),
		})
		if err != nil {
			var apiErr *infrai.APIError
			if errors.As(err, &apiErr) && apiErr.StatusCode >= 400 && apiErr.StatusCode < 500 {
				writeJSON(w, apiErr.StatusCode, map[string]string{"error": apiErr.Message, "code": apiErr.Code})
				return
			}
			log.Printf("avatar request failed: %v", err)
			writeJSON(w, http.StatusBadGateway, map[string]string{"error": "avatar processing failed"})
			return
		}
		writeJSON(w, http.StatusOK, result)
	}
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(value); err != nil && !strings.Contains(err.Error(), "closed") {
		log.Printf("write response: %v", err)
	}
}
