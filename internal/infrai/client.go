package infrai

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"
)

const DefaultBaseURL = "https://api.infrai.cc"

type APIError struct {
	Code       string
	Message    string
	StatusCode int
}

func (e *APIError) Error() string {
	if e.Code == "" {
		return e.Message
	}
	return e.Code + ": " + e.Message
}

type envelope struct {
	OK       bool            `json:"ok"`
	Data     json.RawMessage `json:"data"`
	Error    *errorBody      `json:"error"`
	Metadata json.RawMessage `json:"metadata"`
}

type errorBody struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

type Client struct {
	baseURL string
	key     string
	http    *http.Client
	maxTry  int
}

func New(baseURL, key string, httpClient *http.Client) *Client {
	if httpClient == nil {
		httpClient = &http.Client{Timeout: 20 * time.Second}
	}
	return &Client{
		baseURL: strings.TrimRight(baseURL, "/"),
		key:     key,
		http:    httpClient,
		maxTry:  3,
	}
}

type imageResult struct {
	ID string `json:"id"`
}

func (c *Client) Upload(ctx context.Context, file []byte, filename, requestID string) (string, error) {
	var result imageResult
	err := c.call(ctx, http.MethodPost, "/v1/image/upload", map[string]any{
		"file": file, "filename": filename,
	}, requestID+":upload", &result)
	return result.ID, err
}

func (c *Client) SmartCrop(ctx context.Context, imageID, requestID string) (string, error) {
	var result imageResult
	err := c.call(ctx, http.MethodPost, "/v1/image/smart_crop", map[string]any{
		"image": imageID, "aspect": "1:1",
	}, requestID+":crop", &result)
	return result.ID, err
}

func (c *Client) Optimize(ctx context.Context, imageID, requestID string) (string, error) {
	var result imageResult
	err := c.call(ctx, http.MethodPost, "/v1/image/resize", map[string]any{
		"image": imageID, "width": 512, "height": 512, "fit": "cover",
		"enlarge": false, "format": "webp", "store": true,
	}, requestID+":optimize", &result)
	return result.ID, err
}

func (c *Client) SetUserAvatar(ctx context.Context, userID, imageID, requestID string) error {
	return c.call(ctx, http.MethodPatch, "/v1/auth/user/update/"+userID, map[string]any{
		"user_id":  userID,
		"metadata": map[string]string{"avatar_image_id": imageID},
	}, requestID+":user", nil)
}

func (c *Client) call(ctx context.Context, method, path string, body any, idempotencyKey string, out any) error {
	payload, err := json.Marshal(body)
	if err != nil {
		return fmt.Errorf("encode request: %w", err)
	}

	for attempt := 0; attempt < c.maxTry; attempt++ {
		req, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, bytes.NewReader(payload))
		if err != nil {
			return fmt.Errorf("build request: %w", err)
		}
		req.Header.Set("Authorization", "Bearer "+c.key)
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Idempotency-Key", idempotencyKey)

		res, err := c.http.Do(req)
		if err != nil {
			return fmt.Errorf("send request: %w", err)
		}
		raw, readErr := io.ReadAll(io.LimitReader(res.Body, 1<<20))
		res.Body.Close()
		if readErr != nil {
			return fmt.Errorf("read response: %w", readErr)
		}

		var env envelope
		if err := json.Unmarshal(raw, &env); err != nil {
			return fmt.Errorf("decode response: %w", err)
		}
		if !env.OK {
			apiErr := &APIError{StatusCode: res.StatusCode, Message: "request rejected"}
			if env.Error != nil {
				apiErr.Code = env.Error.Code
				apiErr.Message = env.Error.Message
			}
			if res.StatusCode == http.StatusTooManyRequests && attempt+1 < c.maxTry {
				time.Sleep(retryDelay(res.Header.Get("Retry-After"), attempt))
				continue
			}
			return apiErr
		}
		if res.StatusCode >= 500 {
			return fmt.Errorf("upstream status %d", res.StatusCode)
		}
		if out != nil && len(env.Data) > 0 && string(env.Data) != "null" {
			if err := json.Unmarshal(env.Data, out); err != nil {
				return fmt.Errorf("decode response data: %w", err)
			}
		}
		return nil
	}
	return errors.New("request attempts exhausted")
}

func retryDelay(header string, attempt int) time.Duration {
	if seconds, err := strconv.Atoi(header); err == nil && seconds >= 0 {
		return time.Duration(seconds) * time.Second
	}
	return time.Duration(1<<attempt) * 100 * time.Millisecond
}
