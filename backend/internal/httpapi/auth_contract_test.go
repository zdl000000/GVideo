package httpapi

import (
	"encoding/json"
	"net/http"
	"reflect"
	"sort"
	"testing"
)

// authEndpointContract is the executable registry for docs/auth-api-contract.md.
// Production adoption of the object error envelope remains a separate task.
type authEndpointContract struct {
	Name            string
	Method          string
	Path            string
	SuccessStatus   int
	RequiresSession bool
	RequiresCSRF    bool
	RequestFields   []string
	ResponseFields  []string
	ErrorCodes      []string
}

var authContractRegistry = []authEndpointContract{
	{
		Name:           "register",
		Method:         http.MethodPost,
		Path:           "/api/v1/auth/register",
		SuccessStatus:  http.StatusCreated,
		RequestFields:  []string{"password", "username"},
		ResponseFields: []string{"csrf_token", "user"},
		ErrorCodes:     []string{"conflict", "internal_error", "invalid_input", "invalid_request"},
	},
	{
		Name:           "login",
		Method:         http.MethodPost,
		Path:           "/api/v1/auth/login",
		SuccessStatus:  http.StatusOK,
		RequestFields:  []string{"password", "username"},
		ResponseFields: []string{"csrf_token", "user"},
		ErrorCodes:     []string{"internal_error", "invalid_credentials", "invalid_request"},
	},
	{
		Name:            "logout",
		Method:          http.MethodPost,
		Path:            "/api/v1/auth/logout",
		SuccessStatus:   http.StatusOK,
		RequiresSession: true,
		RequiresCSRF:    true,
		ResponseFields:  []string{"logged_out"},
		ErrorCodes:      []string{"authentication_required", "csrf_failed", "internal_error"},
	},
	{
		Name:            "me",
		Method:          http.MethodGet,
		Path:            "/api/v1/auth/me",
		SuccessStatus:   http.StatusOK,
		RequiresSession: true,
		ResponseFields:  []string{"csrf_token", "user"},
		ErrorCodes:      []string{"authentication_required", "internal_error"},
	},
}

type authContractUser struct {
	ID        int64  `json:"id"`
	Username  string `json:"username"`
	Bio       string `json:"bio"`
	AvatarURL string `json:"avatar_url"`
	IsAdmin   bool   `json:"is_admin"`
	CreatedAt string `json:"created_at"`
}

type authContractPayload struct {
	User      authContractUser `json:"user"`
	CSRFToken string           `json:"csrf_token"`
}

type authSessionCookieContract struct {
	Name     string
	Path     string
	HTTPOnly bool
	SameSite http.SameSite
}

var authCookieContract = authSessionCookieContract{
	Name:     "gvideo_session",
	Path:     "/",
	HTTPOnly: true,
	SameSite: http.SameSiteLaxMode,
}

func TestAuthContractRegistry(t *testing.T) {
	errorStatuses := make(map[string]int, len(errorContractRegistry))
	for _, entry := range errorContractRegistry {
		errorStatuses[entry.Code] = entry.Status
	}

	seenNames := make(map[string]struct{}, len(authContractRegistry))
	seenOperations := make(map[string]struct{}, len(authContractRegistry))
	for _, endpoint := range authContractRegistry {
		endpoint := endpoint
		t.Run(endpoint.Name, func(t *testing.T) {
			if _, exists := seenNames[endpoint.Name]; exists {
				t.Fatalf("duplicate auth contract name %q", endpoint.Name)
			}
			seenNames[endpoint.Name] = struct{}{}

			operation := endpoint.Method + " " + endpoint.Path
			if _, exists := seenOperations[operation]; exists {
				t.Fatalf("duplicate auth operation %q", operation)
			}
			seenOperations[operation] = struct{}{}

			if endpoint.RequiresCSRF && !endpoint.RequiresSession {
				t.Fatal("CSRF-protected auth operation must require a session")
			}
			assertSortedUnique(t, "request fields", endpoint.RequestFields)
			assertSortedUnique(t, "response fields", endpoint.ResponseFields)
			assertSortedUnique(t, "error codes", endpoint.ErrorCodes)

			for _, code := range endpoint.ErrorCodes {
				status, exists := errorStatuses[code]
				if !exists {
					t.Fatalf("error code %q is absent from the API error contract", code)
				}
				if status < http.StatusBadRequest {
					t.Fatalf("error code %q has non-error status %d", code, status)
				}
			}
		})
	}

	wantOperations := []string{
		"GET /api/v1/auth/me",
		"POST /api/v1/auth/login",
		"POST /api/v1/auth/logout",
		"POST /api/v1/auth/register",
	}
	gotOperations := make([]string, 0, len(seenOperations))
	for operation := range seenOperations {
		gotOperations = append(gotOperations, operation)
	}
	sort.Strings(gotOperations)
	if !reflect.DeepEqual(gotOperations, wantOperations) {
		t.Fatalf("auth operation registry mismatch\n got: %#v\nwant: %#v", gotOperations, wantOperations)
	}
}

func TestAuthContractPayloadShape(t *testing.T) {
	payload := authContractPayload{
		User: authContractUser{
			ID:        42,
			Username:  "contract_user",
			Bio:       "",
			AvatarURL: "",
			IsAdmin:   false,
			CreatedAt: "2026-09-10T00:00:00Z",
		},
		CSRFToken: "opaque-csrf-token",
	}
	encoded, err := json.Marshal(payload)
	if err != nil {
		t.Fatal(err)
	}

	var got map[string]any
	if err := json.Unmarshal(encoded, &got); err != nil {
		t.Fatal(err)
	}
	want := map[string]any{
		"csrf_token": "opaque-csrf-token",
		"user": map[string]any{
			"id":         float64(42),
			"username":   "contract_user",
			"bio":        "",
			"avatar_url": "",
			"is_admin":   false,
			"created_at": "2026-09-10T00:00:00Z",
		},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("auth payload mismatch\n got: %#v\nwant: %#v", got, want)
	}
}

func TestAuthSessionCookieAndCSRFContract(t *testing.T) {
	wantCookie := authSessionCookieContract{
		Name:     "gvideo_session",
		Path:     "/",
		HTTPOnly: true,
		SameSite: http.SameSiteLaxMode,
	}
	if !reflect.DeepEqual(authCookieContract, wantCookie) {
		t.Fatalf("auth cookie contract mismatch\n got: %#v\nwant: %#v", authCookieContract, wantCookie)
	}

	for _, endpoint := range authContractRegistry {
		if endpoint.Name == "logout" && (!endpoint.RequiresSession || !endpoint.RequiresCSRF) {
			t.Fatal("logout must require both the session cookie and X-CSRF-Token")
		}
		if endpoint.Name == "me" && endpoint.RequiresCSRF {
			t.Fatal("GET auth/me must not require X-CSRF-Token")
		}
	}
}

func assertSortedUnique(t *testing.T, label string, values []string) {
	t.Helper()
	if !sort.StringsAreSorted(values) {
		t.Fatalf("%s must be sorted: %#v", label, values)
	}
	for index := 1; index < len(values); index++ {
		if values[index] == values[index-1] {
			t.Fatalf("%s contains duplicate %q", label, values[index])
		}
	}
}
