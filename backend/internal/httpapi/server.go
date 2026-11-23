package httpapi

import (
	"context"
	"crypto/hmac"
	"encoding/json"
	"errors"
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/limiter"
	"github.com/gofiber/fiber/v2/middleware/recover"
	"github.com/gofiber/fiber/v2/middleware/requestid"
	"github.com/jackc/pgx/v5"
	"github.com/rs/zerolog/log"
	"io"
	"net"
	"net/url"
	"os"
	"saturnita/pulse/internal/identity"
	"saturnita/pulse/internal/monitoring"
	"saturnita/pulse/internal/pulse"
	"strings"
	"time"
)

const cookie = "saturnita_session"

func edgeTrusted() bool { return os.Getenv("PULSE_EDGE") == "caddy" }
func clientAddress(c *fiber.Ctx) string {
	if edgeTrusted() {
		return c.Get("X-Pulse-IP")
	}
	return c.IP()
}

func decode(c *fiber.Ctx, v any) error {
	if !c.Is("json") {
		return identity.ErrInvalid
	}
	d := json.NewDecoder(strings.NewReader(string(c.Body())))
	d.DisallowUnknownFields()
	if e := d.Decode(v); e != nil {
		return identity.ErrInvalid
	}
	if d.Decode(&struct{}{}) != io.EOF {
		return identity.ErrInvalid
	}
	return nil
}
func failure(c *fiber.Ctx, e error) error {
	status, code, msg := 500, "internal", "Ошибка сервера"
	switch {
	case errors.Is(e, monitoring.ErrState):
		status, code, msg = 409, "monitor_conflict", "Проверьте статус, квоту и интервал ручной проверки"
	case errors.Is(e, identity.ErrInvalid), errors.Is(e, monitoring.ErrInput):
		status, code, msg = 422, "validation_failed", "Проверьте введённые данные"
	case errors.Is(e, identity.ErrAuth):
		status, code, msg = 401, "unauthenticated", "Требуется вход или неверны учётные данные"
	case errors.Is(e, identity.ErrConflict):
		status, code, msg = 409, "conflict", "Адрес уже используется"
	case errors.Is(e, pgx.ErrNoRows):
		status, code, msg = 404, "not_found", "Запись не найдена"
	default:
		var f *fiber.Error
		if errors.As(e, &f) {
			status, code, msg = f.Code, "http_error", f.Message
		}
	}
	return c.Status(status).JSON(fiber.Map{"error": fiber.Map{"code": code, "message": msg, "request_id": c.GetRespHeader("X-Request-ID")}})
}
func New(store *identity.Store, origin string, root context.Context) *fiber.App {
	app := fiber.New(fiber.Config{BodyLimit: 32768, ReadTimeout: 10 * time.Second, WriteTimeout: 15 * time.Second, DisableStartupMessage: true, ErrorHandler: failure})
	app.Use(recover.New(), requestid.New(), func(c *fiber.Ctx) error {
		ctx, cancel := context.WithTimeout(root, 8*time.Second)
		defer cancel()
		c.SetUserContext(ctx)
		c.Set("Cache-Control", "no-store")
		e := c.Next()
		log.Info().Str("request_id", c.GetRespHeader("X-Request-ID")).Str("method", c.Method()).Bool("handler_error", e != nil).Msg("request")
		return e
	})
	var product *product
	if store != nil {
		product = newProduct(store.DB, origin)
	}
	u, _ := url.Parse(origin)
	app.Use(func(c *fiber.Ctx) error {
		host := string(c.Request().Header.Host())
		if h, _, err := net.SplitHostPort(host); err == nil {
			host = h
		}
		if pulse.Host(host) == u.Hostname() {
			return c.Next()
		}
		if host == "backend" && c.Path() == "/internal/caddy/allow-domain" && product != nil && c.Method() == "GET" {
			var allowed bool
			if e := product.db.QueryRow(c.UserContext(), "select exists(select from domains where hostname=$1 and kind='custom' and status='active')", pulse.Host(c.Query("domain"))).Scan(&allowed); e != nil {
				return e
			}
			if !allowed {
				return fiber.ErrForbidden
			}
			return c.SendStatus(200)
		}
		if product == nil {
			return fiber.ErrNotFound
		}
		return product.redirect(c, product.canonical(host))
	})
	if product != nil {
		product.collector(app)
	}
	app.Get("/healthz", func(c *fiber.Ctx) error { return c.JSON(fiber.Map{"status": "ok"}) })
	app.Get("/readyz", func(c *fiber.Ctx) error {
		var ok bool
		if e := store.DB.QueryRow(c.UserContext(), "SELECT version=5 AND NOT dirty FROM schema_migrations").Scan(&ok); e != nil || !ok {
			return fiber.ErrServiceUnavailable
		}
		return c.JSON(fiber.Map{"status": "ready"})
	})
	api := app.Group("/api/v1", func(c *fiber.Ctx) error {
		if c.Method() != "GET" && c.Get("Origin") != origin {
			return fiber.ErrForbidden
		}
		return c.Next()
	})
	sessionCookie := func(c *fiber.Ctx, value string, age int) {
		c.Cookie(&fiber.Cookie{Name: cookie, Value: value, HTTPOnly: true, Secure: strings.HasPrefix(origin, "https://"), SameSite: "Lax", Path: "/", MaxAge: age})
	}
	limit := limiter.New(limiter.Config{Max: 20, KeyGenerator: clientAddress, Expiration: time.Minute, LimitReached: func(c *fiber.Ctx) error { return fiber.ErrTooManyRequests }})
	for _, route := range []string{"login", "register"} {
		api.Post("/auth/"+route, limit, func(c *fiber.Ctx) error {
			var d struct {
				Email    string
				Password string
				Name     string `json:"display_name"`
			}
			if e := decode(c, &d); e != nil {
				return e
			}
			u, token, e := store.Authenticate(c.UserContext(), route == "register", d.Email, d.Password, d.Name, c.Get("User-Agent"))
			if e != nil {
				return e
			}
			sessionCookie(c, token, 14*86400)
			if route == "register" {
				c.Status(201)
			}
			return c.JSON(u)
		})
	}
	api.Use(func(c *fiber.Ctx) error {
		t := c.Cookies(cookie)
		p, e := store.Resolve(c.UserContext(), t)
		if e != nil {
			return e
		}
		if c.Method() != "GET" && !hmac.Equal([]byte(identity.CSRF(t)), []byte(c.Get("X-CSRF-Token"))) {
			return fiber.ErrForbidden
		}
		c.Locals("principal", p)
		return c.Next()
	})
	principal := func(c *fiber.Ctx) identity.Principal { return c.Locals("principal").(identity.Principal) }
	api.Get("/me", func(c *fiber.Ctx) error {
		return c.JSON(struct {
			identity.User
			CSRF string `json:"csrf_token"`
		}{principal(c).User, identity.CSRF(c.Cookies(cookie))})
	})
	api.Patch("/me", func(c *fiber.Ctx) error {
		var d identity.Profile
		if e := decode(c, &d); e != nil {
			return e
		}
		u, e := store.Update(c.UserContext(), principal(c), d)
		if e != nil {
			return e
		}
		return c.JSON(u)
	})
	api.Put("/me/password", limit, func(c *fiber.Ctx) error {
		var d struct {
			Current string `json:"current_password"`
			Next    string `json:"new_password"`
		}
		if e := decode(c, &d); e != nil {
			return e
		}
		if e := store.Password(c.UserContext(), principal(c), d.Current, d.Next); e != nil {
			return e
		}
		return c.SendStatus(204)
	})
	api.Get("/me/sessions", func(c *fiber.Ctx) error {
		v, e := store.Sessions(c.UserContext(), principal(c))
		if e != nil {
			return e
		}
		return c.JSON(fiber.Map{"items": v, "next_cursor": nil})
	})
	revoke := func(c *fiber.Ctx, id string) error {
		p := principal(c)
		if e := store.Revoke(c.UserContext(), p, id); e != nil {
			return e
		}
		if id == p.Session {
			sessionCookie(c, "", -1)
		}
		return c.SendStatus(204)
	}
	api.Delete("/me/sessions/:id", func(c *fiber.Ctx) error { return revoke(c, c.Params("id")) })
	api.Post("/auth/logout", func(c *fiber.Ctx) error { return revoke(c, principal(c).Session) })
	if product != nil {
		product.private(api)
	}
	return app
}
