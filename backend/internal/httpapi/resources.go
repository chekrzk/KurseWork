package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"net"
	"net/url"
	"os"
	"saturnita/pulse/internal/identity"
	"saturnita/pulse/internal/monitoring"
	"saturnita/pulse/internal/pulse"
	"strconv"
	"strings"
	"time"
)

type product struct {
	db       *pgxpool.Pool
	origin   *url.URL
	key      []byte
	resolver dnsResolver
}
type dnsResolver interface {
	LookupTXT(context.Context, string) ([]string, error)
	LookupCNAME(context.Context, string) (string, error)
}

func newProduct(db *pgxpool.Pool, origin string) *product {
	u, _ := url.Parse(origin)
	return &product{db: db, origin: u, key: []byte(os.Getenv("ANALYTICS_MASTER_KEY")), resolver: net.DefaultResolver}
}
func (p *product) local() bool { return p.origin.Hostname() == "localhost" }
func (p *product) visitor(c *fiber.Ctx, q queryer, at time.Time, scope string) ([]byte, error) {
	key := p.key
	if len(key) < 32 {
		if !p.local() {
			return nil, fiber.ErrServiceUnavailable
		}
		if e := q.QueryRow(c.UserContext(), "select secret from local_installation_key where singleton").Scan(&key); e != nil {
			return nil, e
		}
	}
	ip := c.IP()
	if os.Getenv("PULSE_EDGE") == "caddy" {
		ip = c.Get("X-Pulse-IP")
	}
	return pulse.Visitor(key, at, scope, ip, c.Get("User-Agent")), nil
}
func (p *product) external(host string) string {
	if p.local() && strings.HasSuffix(host, ".saturnita.ru") {
		host = strings.TrimSuffix(host, ".saturnita.ru") + ".localhost"
	}
	if p.local() {
		if port := p.origin.Port(); port != "" {
			host = net.JoinHostPort(host, port)
		}
		return "http://" + host
	}
	return "https://" + host
}
func (p *product) canonical(host string) string {
	host = pulse.Host(host)
	if p.local() && strings.HasSuffix(host, ".localhost") {
		return strings.TrimSuffix(host, ".localhost") + ".saturnita.ru"
	}
	return host
}
func owner(c *fiber.Ctx) string { return c.Locals("principal").(identity.Principal).User.ID }
func pathID(c *fiber.Ctx) (string, error) {
	id, e := uuid.Parse(c.Params("id"))
	if e != nil {
		return "", pgx.ErrNoRows
	}
	return id.String(), nil
}
func dbError(e error) error {
	var pe *pgconn.PgError
	if errors.As(e, &pe) {
		switch pe.Code {
		case "23503", "23505":
			return fiber.NewError(409, "Имя занято или ресурс используется")
		case "23514", "22001", "22P02":
			return identity.ErrInvalid
		}
	}
	return e
}

type queryer interface {
	Query(context.Context, string, ...any) (pgx.Rows, error)
	QueryRow(context.Context, string, ...any) pgx.Row
}

func list(c *fiber.Ctx, q queryer, sql string, args ...any) error {
	rows, e := q.Query(c.UserContext(), sql, args...)
	if e != nil {
		return e
	}
	items, e := pgx.CollectRows(rows, pgx.RowTo[json.RawMessage])
	if e != nil {
		return e
	}
	return c.JSON(fiber.Map{"items": items})
}
func object(c *fiber.Ctx, q queryer, sql string, args ...any) error {
	var data json.RawMessage
	if e := q.QueryRow(c.UserContext(), sql, args...).Scan(&data); e != nil {
		return dbError(e)
	}
	return c.JSON(data)
}
func (p *product) private(api fiber.Router) {
	p.monitors(api)
	api.Get("/domains", p.domains)
	api.Get("/domains/:id", p.domain)
	api.Post("/domains/subdomains", p.claim)
	api.Post("/domains/custom", p.claim)
	api.Post("/domains/:id/verify", p.verify)
	api.Post("/domains/:id/activate", p.activate)
	api.Post("/domains/:id/verification/rotate", p.rotate)
	api.Delete("/domains/:id", p.deleteDomain)
	api.Get("/links", p.links)
	api.Post("/links", p.saveLink)
	api.Get("/links/:id", p.link)
	api.Patch("/links/:id", p.saveLink)
	api.Delete("/links/:id", p.deleteLink)
	api.Get("/link-analytics", p.report)
	api.Get("/links/:id/analytics", p.report)
	api.Get("/links/:id/clicks", p.clicks)
	api.Get("/dashboard/overview", p.report)
	api.Get("/sites", p.sites)
	api.Post("/sites", p.saveSite)
	api.Get("/sites/:id", p.site)
	api.Patch("/sites/:id", p.saveSite)
	api.Put("/sites/:id/origins", p.saveSite)
	api.Delete("/sites/:id", p.deleteSite)
	api.Get("/sites/:id/snippet", p.snippet)
	api.Get("/sites/:id/analytics", p.report)
}

type linkInput struct {
	Monitor *bool             `json:"monitor_target"`
	Name    string            `json:"name"`
	Target  string            `json:"target_url"`
	Slug    string            `json:"slug"`
	Domain  string            `json:"domain_id"`
	Status  string            `json:"status"`
	Expires *time.Time        `json:"expires_at"`
	Limit   *int64            `json:"click_limit"`
	UTM     map[string]string `json:"utm"`
}

const linkCols = "l.id,l.name,l.slug,l.target_url,l.domain_id,l.status,l.expires_at,l.click_limit,l.click_count,l.utm,l.created_at,l.updated_at,d.hostname,(select id from monitors where link_id=l.id) monitor_id"

func (p *product) links(c *fiber.Ctx) error {
	limit, e := strconv.Atoi(c.Query("limit", "25"))
	if e != nil || limit < 1 || limit > 100 {
		return identity.ErrInvalid
	}
	cursor := c.Query("cursor")
	if cursor != "" {
		if _, e := uuid.Parse(cursor); e != nil {
			return identity.ErrInvalid
		}
	}
	column := map[string]string{"new": "created_at", "clicks": "click_count", "name": "name"}[c.Query("sort", "new")]
	if column == "" {
		return identity.ErrInvalid
	}
	order, cmp := " desc", "<"
	if column == "name" {
		order, cmp = " asc", ">"
	}
	after := "(l." + column + ",l.id)" + cmp + "(select " + column + ",id from links where user_id=$1 and id=nullif($4,'')::uuid)"
	rows, e := p.db.Query(c.UserContext(), "select to_jsonb(r) from (select "+linkCols+" from links l join domains d on d.id=l.domain_id where l.user_id=$1 and ($2='' or l.status=$2) and ($3='' or concat_ws(' ',l.name,l.slug,l.target_url) ilike '%'||$3||'%') and ($4='' or "+after+") and ($6='' or l.domain_id=nullif($6,'')::uuid) order by l."+column+" "+order+",l.id "+order+" limit $5) r", owner(c), c.Query("status"), c.Query("query"), cursor, limit+1, c.Query("domain_id"))
	if e != nil {
		return dbError(e)
	}
	defer rows.Close()
	items := []map[string]any{}
	var next any
	for rows.Next() {
		var m map[string]any
		if e = rows.Scan(&m); e != nil {
			return e
		}
		if len(items) == limit {
			next = items[limit-1]["id"]
			break
		}
		m["short_url"] = p.external(m["hostname"].(string)) + "/" + m["slug"].(string)
		items = append(items, m)
	}
	if e = rows.Err(); e != nil {
		return e
	}
	return c.JSON(fiber.Map{"items": items, "next_cursor": next})
}
func (p *product) sendLink(c *fiber.Ctx, id string) error {
	var m map[string]any
	e := p.db.QueryRow(c.UserContext(), "select to_jsonb(r) from (select "+linkCols+" from links l join domains d on d.id=l.domain_id where l.user_id=$1 and l.id=$2) r", owner(c), id).Scan(&m)
	if e != nil {
		return e
	}
	m["short_url"] = p.external(m["hostname"].(string)) + "/" + m["slug"].(string)
	return c.JSON(m)
}
func (p *product) link(c *fiber.Ctx) error {
	id, e := pathID(c)
	if e != nil {
		return e
	}
	return p.sendLink(c, id)
}
func (p *product) saveLink(c *fiber.Ctx) error {
	ctx := c.UserContext()
	tx, e := p.db.Begin(ctx)
	if e != nil {
		return e
	}
	defer tx.Rollback(ctx)
	// Serialize hostname registration and link configuration to prevent future internal cycles.
	if _, e = tx.Exec(ctx, "select pg_advisory_xact_lock(7184502)"); e != nil {
		return e
	}
	id := c.Params("id")
	d := linkInput{Status: "active", UTM: map[string]string{}}
	if id != "" {
		if _, e = pathID(c); e != nil {
			return e
		}
		e = tx.QueryRow(ctx, "select name,target_url,slug,domain_id,status,expires_at,click_limit,utm from links where user_id=$1 and id=$2 for update", owner(c), id).Scan(&d.Name, &d.Target, &d.Slug, &d.Domain, &d.Status, &d.Expires, &d.Limit, &d.UTM)
		if e != nil {
			return e
		}
	}
	oldSlug, oldDomain := d.Slug, d.Domain
	if e = decode(c, &d); e != nil {
		return e
	}
	if id != "" && (oldSlug != d.Slug || oldDomain != d.Domain) {
		return identity.ErrInvalid
	}
	d.Name = strings.TrimSpace(d.Name)
	d.Slug = strings.ToLower(d.Slug)
	if d.Slug == "" {
		d.Slug = pulse.Token(5)
	}
	u, ok := pulse.Target(d.Target)
	if !ok || !pulse.Slug(d.Slug) || d.Name == "" || len([]rune(d.Name)) > 160 || !pulse.ValidUTM(d.UTM) {
		return identity.ErrInvalid
	}
	if d.UTM == nil {
		d.UTM = map[string]string{}
	}
	host := p.canonical(u.Hostname())
	var allowed bool
	e = tx.QueryRow(ctx, "select exists(select from domains where id=$1 and (user_id=$2 or kind='system') and status='active') and not exists(select from domains where hostname=$3 and status='active')", d.Domain, owner(c), host).Scan(&allowed)
	if e != nil {
		return dbError(e)
	}
	if !allowed {
		return identity.ErrInvalid
	}
	if id == "" {
		e = tx.QueryRow(ctx, "insert into links(user_id,domain_id,name,slug,target_url,target_host,utm,status,expires_at,click_limit) values($1,$2,$3,$4,$5,$6,$7,$8,$9,$10) returning id", owner(c), d.Domain, d.Name, d.Slug, d.Target, host, d.UTM, d.Status, d.Expires, d.Limit).Scan(&id)
		c.Status(201)
	} else {
		_, e = tx.Exec(ctx, "update links set name=$1,target_url=$2,target_host=$3,utm=$4,status=$5,expires_at=$6,click_limit=$7,updated_at=now() where id=$8", d.Name, d.Target, host, d.UTM, d.Status, d.Expires, d.Limit, id)
	}
	if e != nil {
		return dbError(e)
	}
	if e = monitoring.SyncLink(ctx, tx, owner(c), id, d.Name, d.Target, d.Status, d.Monitor); e != nil {
		return e
	}
	if e = tx.Commit(ctx); e != nil {
		return e
	}
	return p.sendLink(c, id)
}
func (p *product) remove(c *fiber.Ctx, table string) error {
	id, e := pathID(c)
	if e != nil {
		return e
	}
	tag, e := p.db.Exec(c.UserContext(), "delete from "+table+" where id=$1 and user_id=$2", id, owner(c))
	if e != nil {
		return dbError(e)
	}
	if tag.RowsAffected() == 0 {
		return pgx.ErrNoRows
	}
	return c.SendStatus(204)
}
func (p *product) deleteLink(c *fiber.Ctx) error {
	if c.Query("confirm") != "true" {
		return identity.ErrInvalid
	}
	return p.remove(c, "links")
}
func (p *product) redirect(c *fiber.Ctx, host string) error {
	if c.Method() != "GET" && c.Method() != "HEAD" {
		return fiber.ErrNotFound
	}
	slug := strings.ToLower(strings.TrimPrefix(c.Path(), "/"))
	if !pulse.Slug(slug) {
		return fiber.ErrNotFound
	}
	ctx := c.UserContext()
	tx, e := p.db.Begin(ctx)
	if e != nil {
		return e
	}
	defer tx.Rollback(ctx)
	var id, owner, raw, status string
	var utm map[string]string
	var available bool
	e = tx.QueryRow(ctx, `select l.id,l.user_id,l.target_url,l.utm,l.status,(expires_at is null or expires_at>now()) and (click_limit is null or click_count<click_limit) and not exists(select from domains target where target.hostname=l.target_host and target.status='active') from links l join domains d on d.id=l.domain_id where d.hostname=$1 and d.status='active' and slug=$2 for update of l`, host, slug).Scan(&id, &owner, &raw, &utm, &status, &available)
	if e != nil {
		return e
	}
	if status != "active" || !available {
		return fiber.ErrGone
	}
	location := pulse.Location(raw, utm)
	if c.Method() == "GET" {
		at := time.Now().UTC()
		hash, e := p.visitor(c, tx, at, "link:"+owner)
		if e != nil {
			return e
		}
		device, browser, osName, bot := pulse.Agent(c.Get("User-Agent"))
		u, _ := url.Parse(location)
		q := u.Query()
		_, e = tx.Exec(ctx, "insert into link_clicks(link_id,occurred_at,referrer,source,medium,campaign,device,browser,os,is_bot,visitor_hash) values($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11)", id, at, pulse.Ref(c.Get("Referer")), q.Get("utm_source"), q.Get("utm_medium"), q.Get("utm_campaign"), device, browser, osName, bot, hash)
		if e != nil {
			return e
		}
		if _, e = tx.Exec(ctx, "update links set click_count=click_count+1 where id=$1", id); e != nil {
			return e
		}
	}
	if e = tx.Commit(ctx); e != nil {
		return e
	}
	c.Set("Cache-Control", "no-store")
	c.Set("Location", location)
	return c.SendStatus(302)
}
