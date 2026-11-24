package httpapi

import (
	"encoding/json"
	"errors"
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/limiter"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"html"
	"net/url"
	"regexp"
	"saturnita/pulse/internal/identity"
	"saturnita/pulse/internal/pulse"
	"strings"
	"time"
)

const siteCols = "s.id,s.public_id,s.name,s.hostname,s.status,s.timezone,s.created_at,array(select origin from site_origins where site_id=s.id order by origin) origins"

func (p *product) sites(c *fiber.Ctx) error {
	return list(c, p.db, "select to_jsonb(r) from (select "+siteCols+" from sites s where s.user_id=$1 order by s.created_at desc) r", owner(c))
}
func (p *product) site(c *fiber.Ctx) error {
	id, e := pathID(c)
	if e != nil {
		return e
	}
	return p.sendSite(c, id)
}
func (p *product) sendSite(c *fiber.Ctx, id string) error {
	return object(c, p.db, "select to_jsonb(r) from (select "+siteCols+" from sites s where s.id=$1 and s.user_id=$2) r", id, owner(c))
}
func (p *product) deleteSite(c *fiber.Ctx) error { return p.remove(c, "sites") }
func (p *product) snippet(c *fiber.Ctx) error {
	id, e := pathID(c)
	if e != nil {
		return e
	}
	var public string
	e = p.db.QueryRow(c.UserContext(), "select public_id from sites where id=$1 and user_id=$2", id, owner(c)).Scan(&public)
	if e != nil {
		return e
	}
	return c.JSON(fiber.Map{"snippet": "<script defer src=\"" + html.EscapeString(p.origin.String()) + "/tracker.js\" data-site-id=\"" + public + "\"></script>"})
}
func (p *product) saveSite(c *fiber.Ctx) error {
	ctx := c.UserContext()
	tx, e := p.db.Begin(ctx)
	if e != nil {
		return e
	}
	defer tx.Rollback(ctx)
	id := c.Params("id")
	var d struct {
		Name, Hostname, Timezone, Status string
		Origins                          []string
	}
	d.Timezone = "UTC"
	d.Status = "active"
	if id != "" {
		if _, e = pathID(c); e != nil {
			return e
		}
		e = tx.QueryRow(ctx, "select name,hostname,timezone,status,array(select origin from site_origins where site_id=$1) from sites where id=$1 and user_id=$2 for update", id, owner(c)).Scan(&d.Name, &d.Hostname, &d.Timezone, &d.Status, &d.Origins)
		if e != nil {
			return e
		}
	}
	if c.Method() == "PUT" {
		var origins struct{ Origins []string }
		if e = decode(c, &origins); e != nil {
			return e
		}
		d.Origins = origins.Origins
	} else if e = decode(c, &d); e != nil {
		return e
	}
	d.Hostname = pulse.Host(d.Hostname)
	_, e = time.LoadLocation(d.Timezone)
	if e != nil || d.Timezone == "" || len(d.Origins) < 1 || len(d.Origins) > 16 || strings.TrimSpace(d.Name) == "" || len(d.Name) > 160 || (!pulse.ValidHost(d.Hostname) && !(p.local() && d.Hostname == "localhost")) {
		return identity.ErrInvalid
	}
	for _, origin := range d.Origins {
		u, ok := pulse.Target(origin)
		if !ok || len(origin) > 512 || origin != u.Scheme+"://"+u.Host || strings.Contains(u.Host, "*") || (!pulse.ValidHost(u.Hostname()) && !(p.local() && u.Hostname() == "localhost")) {
			return identity.ErrInvalid
		}
	}
	if id == "" {
		e = tx.QueryRow(ctx, "insert into sites(user_id,name,hostname,timezone,status) values($1,$2,$3,$4,$5) returning id", owner(c), d.Name, d.Hostname, d.Timezone, d.Status).Scan(&id)
		c.Status(201)
	} else {
		_, e = tx.Exec(ctx, "update sites set name=$1,hostname=$2,timezone=$3,status=$4 where id=$5", d.Name, d.Hostname, d.Timezone, d.Status, id)
	}
	if e != nil {
		return dbError(e)
	}
	if _, e = tx.Exec(ctx, "delete from site_origins where site_id=$1", id); e != nil {
		return e
	}
	if _, e = tx.Exec(ctx, "insert into site_origins select $1,unnest($2::text[]) on conflict do nothing", id, d.Origins); e != nil {
		return e
	}
	if e = tx.Commit(ctx); e != nil {
		return e
	}
	return p.sendSite(c, id)
}

type event struct {
	ID                   string `json:"event_id"`
	Site                 string `json:"site_id"`
	Kind                 string `json:"type"`
	Name, Path, Referrer string
	Occurred             time.Time `json:"occurred_at"`
	UTM                  map[string]string
	Properties           map[string]any
}

var eventName = regexp.MustCompile("^[a-zA-Z][a-zA-Z0-9_.-]{0,79}$")

func (p *product) collector(app *fiber.App) {
	limit := limiter.New(limiter.Config{Max: 240, Expiration: time.Minute, KeyGenerator: func(c *fiber.Ctx) string {
		if c.Get("X-Pulse-IP") != "" && edgeTrusted() {
			return c.Get("X-Pulse-IP")
		}
		return c.IP()
	}, LimitReached: func(c *fiber.Ctx) error { return fiber.ErrTooManyRequests }})
	siteLimit := limiter.New(limiter.Config{Max: 2400, Expiration: time.Minute, KeyGenerator: func(c *fiber.Ctx) string { return c.Locals("collectSite").(string) }, LimitReached: func(c *fiber.Ctx) error { return fiber.ErrTooManyRequests }})
	app.Options("/api/v1/collect", limit, p.collectOrigin)
	app.Post("/api/v1/collect", limit, p.collectOrigin, siteLimit, p.collect)
}
func (p *product) collectOrigin(c *fiber.Ctx) error {
	origin := c.Get("Origin")
	public := c.Query("site_id")
	if _, e := uuid.Parse(public); e != nil || origin == "" {
		return fiber.ErrForbidden
	}
	var site string
	e := p.db.QueryRow(c.UserContext(), "select s.id from sites s join site_origins o on s.id=o.site_id where s.public_id=$1 and s.status='active' and o.origin=$2", public, origin).Scan(&site)
	if e != nil {
		if errors.Is(e, pgx.ErrNoRows) {
			return fiber.ErrForbidden
		}
		return e
	}
	c.Vary("Origin")
	c.Set("Access-Control-Allow-Origin", origin)
	if c.Method() == "OPTIONS" {
		if c.Get("Access-Control-Request-Method") != "POST" {
			return fiber.ErrForbidden
		}
		c.Set("Access-Control-Allow-Methods", "POST")
		c.Set("Access-Control-Allow-Headers", "Content-Type")
		return c.SendStatus(204)
	}
	c.Locals("collectSite", site)
	return c.Next()
}
func (p *product) collect(c *fiber.Ctx) error {
	public := c.Query("site_id")
	site := c.Locals("collectSite").(string)
	var e error
	var d event
	if e = decode(c, &d); e != nil {
		return e
	}
	now := time.Now().UTC()
	if _, e = uuid.Parse(d.ID); e != nil || d.Site != public || !eventName.MatchString(d.Name) || d.Kind != "pageview" && d.Kind != "custom" || d.Kind == "pageview" && d.Name != "pageview" || len(d.Path) > 2048 || len(d.Properties) > 16 || !pulse.ValidUTM(d.UTM) || d.Occurred.Before(now.Add(-24*time.Hour)) || d.Occurred.After(now.Add(5*time.Minute)) {
		return identity.ErrInvalid
	}
	if d.Path == "" {
		d.Path = "/"
	}
	clean, _, _ := strings.Cut(d.Path, "#")
	u, e := url.ParseRequestURI(clean)
	if e != nil || !strings.HasPrefix(d.Path, "/") || strings.HasPrefix(d.Path, "//") {
		return identity.ErrInvalid
	}
	path := strings.SplitN(u.EscapedPath(), "#", 2)[0]
	for k, v := range d.Properties {
		if !eventName.MatchString(k) {
			return identity.ErrInvalid
		}
		switch val := v.(type) {
		case nil, bool, float64:
		case string:
			if len(val) > 255 {
				return identity.ErrInvalid
			}
		default:
			return identity.ErrInvalid
		}
	}
	if d.Properties == nil {
		d.Properties = map[string]any{}
	}
	props, _ := json.Marshal(d.Properties)
	hash, e := p.visitor(c, p.db, now, "site:"+site)
	if e != nil {
		return e
	}
	device, browser, osName, _ := pulse.Agent(c.Get("User-Agent"))
	_, e = p.db.Exec(c.UserContext(), "insert into web_events(site_id,id,occurred_at,received_at,kind,name,path,referrer,source,medium,campaign,properties,device,browser,os,visitor_hash) values($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16) on conflict(site_id,id) do nothing", site, d.ID, d.Occurred, now, d.Kind, d.Name, path, pulse.Ref(d.Referrer), d.UTM["source"], d.UTM["medium"], d.UTM["campaign"], props, device, browser, osName, hash)
	if e != nil {
		return e
	}
	return c.SendStatus(204)
}
