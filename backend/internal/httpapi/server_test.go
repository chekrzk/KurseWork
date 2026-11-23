package httpapi

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/jackc/pgx/v5/pgxpool"
	"net/http/httptest"
	"os"
	"saturnita/pulse/internal/identity"
	"strings"
	"testing"
	"time"
)

func TestOriginAndLiveness(t *testing.T) {
	app := New(nil, "http://localhost:5173", context.Background())
	for _, tc := range []struct {
		method, path string
		code         int
	}{{"GET", "/healthz", 200}, {"POST", "/api/v1/auth/login", 403}} {
		r := httptest.NewRequest(tc.method, tc.path, nil)
		r.Host = "localhost:5173"
		res, e := app.Test(r)
		if e != nil || res.StatusCode != tc.code {
			t.Fatal(tc, e, res)
		}
		res.Body.Close()
	}
}
func TestIdentityPostgres(t *testing.T) {
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("set TEST_DATABASE_URL to a migrated test database")
	}
	ctx := context.Background()
	db, e := pgxpool.New(ctx, dsn)
	if e != nil {
		t.Fatal(e)
	}
	defer db.Close()
	const origin = "http://localhost:5173"
	app := New(identity.New(db), origin, ctx)
	req := func(method, path, body, cookie, csrf string, want int) (map[string]any, string) {
		t.Helper()
		r := httptest.NewRequest(method, "/api/v1"+path, strings.NewReader(body))
		r.Host = "localhost:5173"
		r.Header.Set("Origin", origin)
		r.Header.Set("Content-Type", "application/json")
		r.Header.Set("Cookie", cookie)
		r.Header.Set("X-CSRF-Token", csrf)
		res, e := app.Test(r, 10000)
		if e != nil {
			t.Fatal(e)
		}
		defer res.Body.Close()
		var data map[string]any
		_ = json.NewDecoder(res.Body).Decode(&data)
		if res.StatusCode != want {
			t.Fatalf("%s %s got %d want %d: %v", method, path, res.StatusCode, want, data)
		}
		return data, strings.Split(res.Header.Get("Set-Cookie"), ";")[0]
	}
	email := fmt.Sprintf("test-%d@example.test", time.Now().UnixNano())
	defer db.Exec(ctx, "DELETE FROM users WHERE email=$1", email)
	body := fmt.Sprintf(`{"email":%q,"password":"example long password","display_name":"Test"}`, email)
	_, cookie := req("POST", "/auth/register", body, "", "", 201)
	me, _ := req("GET", "/me", "", cookie, "", 200)
	csrf := me["csrf_token"].(string)
	req("PATCH", "/me", `{"display_name":"New"}`, cookie, "wrong", 403)
	req("PATCH", "/me", `{"display_name":"New"}`, cookie, csrf, 200)
	_, second := req("POST", "/auth/login", body, "", "", 200)
	req("PUT", "/me/password", `{"current_password":"example long password","new_password":"another long password"}`, cookie, csrf, 204)
	req("GET", "/me", "", second, "", 401)
	req("GET", "/me/sessions", "", cookie, "", 200)
	other := strings.Replace(email, "test-", "other-", 1)
	defer db.Exec(ctx, "DELETE FROM users WHERE email=$1", other)
	_, foreign := req("POST", "/auth/register", strings.Replace(body, email, other, 1), "", "", 201)
	data, _ := req("GET", "/me/sessions", "", foreign, "", 200)
	foreignID := data["items"].([]any)[0].(map[string]any)["id"].(string)
	req("DELETE", "/me/sessions/"+foreignID, "", cookie, csrf, 404)
	req("POST", "/auth/logout", "", cookie, csrf, 204)
	req("GET", "/me", "", cookie, "", 401)
}
