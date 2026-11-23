package httpapi

import (
	"context"
	"encoding/json"
	"github.com/gofiber/fiber/v2"
	"github.com/jackc/pgx/v5/pgxpool"
	"net/http/httptest"
	"os"
	"saturnita/pulse/internal/identity"
	"saturnita/pulse/internal/pulse"
	"strings"
	"testing"
)

type fixtureDNS struct{ token, target string }

func (f *fixtureDNS) LookupTXT(context.Context, string) ([]string, error) {
	return []string{f.token}, nil
}
func (f *fixtureDNS) LookupCNAME(context.Context, string) (string, error) { return f.target, nil }
func TestCustomDomainOwnership(t *testing.T) {
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("requires migrated PostgreSQL")
	}
	ctx := context.Background()
	db, e := pgxpool.New(ctx, dsn)
	if e != nil {
		t.Fatal(e)
	}
	defer db.Close()
	var account string
	host := pulse.Token(8) + ".example.test"
	e = db.QueryRow(ctx, "insert into users(email,display_name,password_hash) values($1,'fixture','unused') returning id", "owner@"+host).Scan(&account)
	if e != nil {
		t.Fatal(e)
	}
	defer db.Exec(ctx, "delete from users where id=$1", account)
	p := newProduct(db, "http://localhost:5173")
	dns := &fixtureDNS{target: "go.saturnita.ru."}
	p.resolver = dns
	app := fiber.New(fiber.Config{ErrorHandler: failure})
	app.Use(func(c *fiber.Ctx) error {
		c.Locals("principal", identity.Principal{User: identity.User{ID: account}})
		return c.Next()
	})
	p.private(app)
	call := func(path, body string, status int) map[string]any {
		t.Helper()
		r := httptest.NewRequest("POST", path, strings.NewReader(body))
		r.Header.Set("Content-Type", "application/json")
		res, e := app.Test(r)
		if e != nil {
			t.Fatal(e)
		}
		defer res.Body.Close()
		var d map[string]any
		json.NewDecoder(res.Body).Decode(&d)
		if res.StatusCode != status {
			t.Fatalf("%s: %d %v", path, res.StatusCode, d)
		}
		return d
	}
	d := call("/domains/custom", `{"hostname":"`+host+`"}`, 201)
	path := "/domains/" + d["id"].(string)
	call(path+"/activate", "", 409)
	call(path+"/verify", "", 422)
	dns.token = d["challenge"].(string)
	call(path+"/verify", "", 200)
	dns.target = "elsewhere.test."
	call(path+"/activate", "", 422)
	dns.target = "go.saturnita.ru."
	if call(path+"/activate", "", 200)["status"] != "active" {
		t.Fatal("expected active")
	}
}
