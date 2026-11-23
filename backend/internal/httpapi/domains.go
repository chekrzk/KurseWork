package httpapi

import (
	"github.com/gofiber/fiber/v2"
	"saturnita/pulse/internal/identity"
	"saturnita/pulse/internal/pulse"
	"strings"
)

const domainFields = "id,hostname,kind,status,created_at,verified_at,challenge,challenge_expires_at"

func (p *product) domains(c *fiber.Ctx) error {
	return list(c, p.db, "select to_jsonb(d) from (select "+domainFields+" from domains where user_id=$1 or kind='system' order by kind desc,hostname) d", owner(c))
}
func (p *product) domain(c *fiber.Ctx) error {
	id, e := pathID(c)
	if e != nil {
		return e
	}
	return object(c, p.db, "select to_jsonb(d) from (select "+domainFields+" from domains where id=$1 and (user_id=$2 or kind='system')) d", id, owner(c))
}
func (p *product) claim(c *fiber.Ctx) error {
	var d struct{ Label, Hostname string }
	if e := decode(c, &d); e != nil {
		return e
	}
	host := pulse.Host(d.Hostname)
	kind, status := "custom", "pending"
	if strings.HasSuffix(c.Path(), "/subdomains") {
		if !pulse.Label(d.Label) {
			return identity.ErrInvalid
		}
		kind, status, host = "subdomain", "active", d.Label+".saturnita.ru"
	} else if !pulse.ValidHost(host) || host == "saturnita.ru" || strings.HasSuffix(host, ".saturnita.ru") || strings.HasSuffix(host, ".localhost") {
		return identity.ErrInvalid
	}
	ctx := c.UserContext()
	tx, e := p.db.Begin(ctx)
	if e != nil {
		return e
	}
	defer tx.Rollback(ctx)
	if _, e = tx.Exec(ctx, "select pg_advisory_xact_lock(7184502)"); e != nil {
		return e
	}
	var id string
	e = tx.QueryRow(ctx, `insert into domains(user_id,hostname,kind,status,challenge,challenge_expires_at) select $1,$2,$3,$4,case when $3='custom' then $5 end,case when $3='custom' then now()+interval '24 hours' end returning id`, owner(c), host, kind, status, pulse.Token(24)).Scan(&id)
	if e != nil {
		return dbError(e)
	}
	if e = tx.Commit(ctx); e != nil {
		return e
	}
	c.Status(201)
	return object(c, p.db, "select to_jsonb(d) from (select "+domainFields+" from domains where id=$1) d", id)
}
func (p *product) rotate(c *fiber.Ctx) error {
	id, e := pathID(c)
	if e != nil {
		return e
	}
	return object(c, p.db, `with d as (update domains set challenge=$3,challenge_expires_at=now()+interval '24 hours',verified_at=null,status='pending' where user_id=$1 and id=$2 and kind='custom' and status<>'active' returning `+domainFields+") select to_jsonb(d) from d", owner(c), id, pulse.Token(24))
}
func (p *product) verify(c *fiber.Ctx) error   { return p.verifyDomain(c, false) }
func (p *product) activate(c *fiber.Ctx) error { return p.verifyDomain(c, true) }
func (p *product) verifyDomain(c *fiber.Ctx, activate bool) error {
	id, e := pathID(c)
	if e != nil {
		return e
	}
	ctx := c.UserContext()
	var host, token, status string
	var fresh bool
	e = p.db.QueryRow(ctx, "select hostname,challenge,status,challenge_expires_at>now() from domains where id=$1 and user_id=$2 and kind='custom'", id, owner(c)).Scan(&host, &token, &status, &fresh)
	if e != nil {
		return e
	}
	if status == "active" {
		return p.domain(c)
	}
	if !fresh || activate && status != "verified" {
		return fiber.NewError(409, "Обновите TXT-проверку домена")
	}
	txt, e := p.resolver.LookupTXT(ctx, "_saturnita."+host+".")
	valid := false
	for _, v := range txt {
		valid = valid || v == token
	}
	if e != nil || !valid {
		_, err := p.db.Exec(ctx, "update domains set status='error' where id=$1 and challenge=$2 and status=$3", id, token, status)
		if err != nil {
			return err
		}
		return fiber.NewError(422, "TXT не найден или значение не совпадает")
	}
	next := "verified"
	if activate {
		target, e := p.resolver.LookupCNAME(ctx, host+".")
		if e != nil || pulse.Host(target) != "go.saturnita.ru" {
			return fiber.NewError(422, "Проверьте CNAME на go.saturnita.ru")
		}
		next = "active"
	}
	tag, e := p.db.Exec(ctx, "update domains set status=$1,verified_at=now() where id=$2 and user_id=$3 and challenge=$4 and status=$5 and challenge_expires_at>now()", next, id, owner(c), token, status)
	if e != nil {
		return e
	}
	if tag.RowsAffected() != 1 {
		return fiber.ErrConflict
	}
	return p.domain(c)
}
func (p *product) deleteDomain(c *fiber.Ctx) error { return p.remove(c, "domains") }
