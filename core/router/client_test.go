package router

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) { return f(req) }

func TestClientListLoadUnload(t *testing.T) {
	requests := []string{}
	router := NewClient("http://router", recordingClient(t, &requests))
	models, err := router.List(context.Background())
	if err != nil || len(models) != 1 || models[0].ID != "demo" {
		t.Fatalf("List() = %#v, %v", models, err)
	}
	if err := router.Load(context.Background(), "demo"); err != nil {
		t.Fatal(err)
	}
	if err := router.Unload(context.Background(), "demo"); err != nil {
		t.Fatal(err)
	}
	if _, err := router.Reload(context.Background()); err != nil {
		t.Fatal(err)
	}
	want := []string{"GET /models", "POST /models/load", "POST /models/unload", "GET /models?reload=1"}
	for i := range want {
		if requests[i] != want[i] {
			t.Fatalf("requests = %v", requests)
		}
	}
}

func TestNewClientProvidesTimedHTTPClient(t *testing.T) {
	client := NewClient("http://router/", nil)
	if client.http == nil || client.http.Timeout <= 0 || client.baseURL != "http://router" {
		t.Fatalf("client = %#v", client)
	}
}

func TestClientIncludesBoundedErrorResponse(t *testing.T) {
	client := NewClient("http://router", &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
		body := strings.Repeat("x", 600)
		return &http.Response{StatusCode: http.StatusBadRequest, Status: "400 Bad Request", Body: io.NopCloser(strings.NewReader(body))}, nil
	})})
	err := client.Load(context.Background(), "demo")
	if err == nil || !strings.Contains(err.Error(), "400 Bad Request: "+strings.Repeat("x", 512)) {
		t.Fatalf("Load() error = %v", err)
	}
	if len(err.Error()) >= 600 {
		t.Fatalf("Load() error was not bounded: %d bytes", len(err.Error()))
	}
}

func TestClientDrainsUnusedResponseBody(t *testing.T) {
	body := strings.NewReader("response body")
	client := NewClient("http://router", &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: http.StatusOK, Status: "200 OK", Body: io.NopCloser(body)}, nil
	})})
	if err := client.Load(context.Background(), "demo"); err != nil {
		t.Fatal(err)
	}
	if body.Len() != 0 {
		t.Fatalf("response has %d unread bytes", body.Len())
	}
}

func recordingClient(t *testing.T, requests *[]string) *http.Client {
	t.Helper()
	return &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		*requests = append(*requests, r.Method+" "+r.URL.RequestURI())
		if r.Method == http.MethodPost {
			var body map[string]string
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body["model"] != "demo" {
				t.Fatalf("body = %v, err = %v", body, err)
			}
		}
		body := `{"success":true}`
		if r.URL.Path == "/models" {
			body = `{"data":[{"id":"demo","status":{"value":"loaded"}}]}`
		}
		return &http.Response{StatusCode: 200, Status: "200 OK", Body: io.NopCloser(strings.NewReader(body)), Header: http.Header{}}, nil
	})}
}
