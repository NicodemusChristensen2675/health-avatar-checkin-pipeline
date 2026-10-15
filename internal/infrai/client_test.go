package infrai

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"
)

func TestAvatarRequestBoundary(t *testing.T) {
	type seenRequest struct {
		method string
		path   string
		body   map[string]any
		key    string
		idem   string
	}
	var seen []seenRequest
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatal(err)
		}
		seen = append(seen, seenRequest{r.Method, r.URL.Path, body, r.Header.Get("Authorization"), r.Header.Get("Idempotency-Key")})
		w.Header().Set("Content-Type", "application/json")
		if r.Method == http.MethodPatch {
			_, _ = w.Write([]byte(`{"ok":true,"data":{},"error":null,"metadata":{}}`))
			return
		}
		_, _ = w.Write([]byte(`{"ok":true,"data":{"id":"next-image"},"error":null,"metadata":{}}`))
	}))
	defer server.Close()

	client := New(server.URL, "test-key", server.Client())
	uploaded, err := client.Upload(context.Background(), []byte("photo"), "patient.jpg", "req-9")
	if err != nil {
		t.Fatal(err)
	}
	cropped, err := client.SmartCrop(context.Background(), uploaded, "req-9")
	if err != nil {
		t.Fatal(err)
	}
	optimized, err := client.Optimize(context.Background(), cropped, "req-9")
	if err != nil {
		t.Fatal(err)
	}
	if err := client.SetUserAvatar(context.Background(), "patient-42", optimized, "req-9"); err != nil {
		t.Fatal(err)
	}

	wantPaths := []string{"/v1/image/upload", "/v1/image/smart_crop", "/v1/image/resize", "/v1/auth/user/update/patient-42"}
	for i, want := range wantPaths {
		if seen[i].path != want {
			t.Fatalf("path %d = %q, want %q", i, seen[i].path, want)
		}
		if seen[i].key != "Bearer test-key" || seen[i].idem == "" {
			t.Fatalf("request %d missing shared auth or idempotency header", i)
		}
	}
	if seen[3].method != http.MethodPatch {
		t.Fatalf("update method = %s", seen[3].method)
	}
	if got := seen[3].body["user_id"]; got != "patient-42" {
		t.Fatalf("update user_id = %#v, want %q", got, "patient-42")
	}
	wantResize := map[string]any{"image": "next-image", "width": float64(512), "height": float64(512), "fit": "cover", "enlarge": false, "format": "webp", "store": true}
	if !reflect.DeepEqual(seen[2].body, wantResize) {
		t.Fatalf("resize body = %#v", seen[2].body)
	}
}

func TestEnvelopeErrorPrecedesStatus(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"ok":false,"data":null,"error":{"code":"BAD_INPUT","message":"image is required"},"metadata":{}}`))
	}))
	defer server.Close()

	_, err := New(server.URL, "test-key", server.Client()).SmartCrop(context.Background(), "", "req-10")
	apiErr, ok := err.(*APIError)
	if !ok || apiErr.Code != "BAD_INPUT" || apiErr.StatusCode != http.StatusBadRequest {
		t.Fatalf("error = %#v", err)
	}
}
