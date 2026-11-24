package identity

import (
	"context"
	"encoding/json"
	"errors"
	"net/mail"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

var ErrInvalid = errors.New("invalid input")
var ErrAuth = errors.New("invalid credentials")
var ErrConflict = errors.New("email unavailable")

type Store struct {
	DB    *pgxpool.Pool
	dummy string
	slots chan struct{}
}

func New(db *pgxpool.Pool) *Store {
	return &Store{db, hashPassword("timing equalizer"), make(chan struct{}, 4)}
}

type User struct {
	ID      string          `json:"id"`
	Email   string          `json:"email"`
	Name    string          `json:"display_name"`
	Zone    string          `json:"timezone"`
	Locale  string          `json:"locale"`
	Pref    json.RawMessage `json:"preferences"`
	Created time.Time       `json:"created_at"`
}
type Principal struct {
	User    User
	Session string
}
type Profile struct {
	Name   *string        `json:"display_name"`
	Zone   *string        `json:"timezone"`
	Locale *string        `json:"locale"`
	Pref   map[string]any `json:"preferences"`
}

func validName(s string) bool     { return strings.TrimSpace(s) != "" && utf8.RuneCountInString(s) <= 120 }
func validPassword(s string) bool { return utf8.RuneCountInString(s) >= 12 && len(s) <= 256 }
func (s *Store) acquire(ctx context.Context) error {
	select {
	case s.slots <- struct{}{}:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}
func (s *Store) Authenticate(ctx context.Context, register bool, email, password, name, ua string) (u User, token string, err error) {
	email = strings.ToLower(strings.TrimSpace(email))
	name = strings.TrimSpace(name)
	addr, e := mail.ParseAddress(email)
	if e != nil || addr.Address != email || len(email) > 320 || len(password) > 256 {
		return u, "", ErrInvalid
	}
	if register && (!validPassword(password) || !validName(name)) {
		return u, "", ErrInvalid
	}
	if err = s.acquire(ctx); err != nil {
		return
	}
	defer func() { <-s.slots }()
	var hash, id string
	if register {
		hash = hashPassword(password)
	} else {
		err = s.DB.QueryRow(ctx, "SELECT id,password_hash FROM users WHERE email=$1", email).Scan(&id, &hash)
		if errors.Is(err, pgx.ErrNoRows) {
			matches(s.dummy, password)
			return u, "", ErrAuth
		}
		if err != nil {
			return
		}
		if !matches(hash, password) {
			return u, "", ErrAuth
		}
	}
	tx, err := s.DB.Begin(ctx)
	if err != nil {
		return
	}
	defer tx.Rollback(ctx)
	const fields = "id,email,display_name,timezone,locale,preferences,created_at"
	var row pgx.Row
	if register {
		row = tx.QueryRow(ctx, "INSERT INTO users(email,display_name,password_hash) VALUES($1,$2,$3) RETURNING "+fields, email, name, hash)
	} else {
		row = tx.QueryRow(ctx, "SELECT "+fields+" FROM users WHERE id=$1 AND password_hash=$2 FOR UPDATE", id, hash)
	}
	err = row.Scan(&u.ID, &u.Email, &u.Name, &u.Zone, &u.Locale, &u.Pref, &u.Created)
	if err != nil {
		var pe *pgconn.PgError
		if errors.As(err, &pe) && pe.Code == "23505" {
			err = ErrConflict
		}
		if errors.Is(err, pgx.ErrNoRows) {
			err = ErrAuth
		}
		return
	}
	token = encoding.EncodeToString(random(32))
	r := []rune(ua)
	if len(r) > 512 {
		r = r[:512]
	}
	_, err = tx.Exec(ctx, "INSERT INTO sessions(user_id,token_hash,user_agent) VALUES($1,$2,$3)", u.ID, digest(token), string(r))
	if err != nil {
		return
	}
	err = tx.Commit(ctx)
	return
}
func (s *Store) Resolve(ctx context.Context, token string) (p Principal, e error) {
	if len(token) != 43 {
		return p, ErrAuth
	}
	e = s.DB.QueryRow(ctx, `UPDATE sessions s SET last_seen_at=now() FROM users u WHERE s.user_id=u.id AND s.token_hash=$1 AND s.revoked_at IS NULL AND s.expires_at>now() RETURNING u.id,u.email,u.display_name,u.timezone,u.locale,u.preferences,u.created_at,s.id`, digest(token)).Scan(&p.User.ID, &p.User.Email, &p.User.Name, &p.User.Zone, &p.User.Locale, &p.User.Pref, &p.User.Created, &p.Session)
	if errors.Is(e, pgx.ErrNoRows) {
		e = ErrAuth
	}
	return
}
func (s *Store) Update(ctx context.Context, p Principal, d Profile) (User, error) {
	u := p.User
	if d.Name != nil && !validName(*d.Name) {
		return u, ErrInvalid
	}
	if d.Zone != nil {
		if _, e := time.LoadLocation(*d.Zone); e != nil || *d.Zone == "" {
			return u, ErrInvalid
		}
	}
	if d.Locale != nil && *d.Locale != "ru" && *d.Locale != "en" {
		return u, ErrInvalid
	}
	var pref []byte
	if d.Pref != nil {
		pref, _ = json.Marshal(d.Pref)
		if len(pref) > 4096 {
			return u, ErrInvalid
		}
	}
	e := s.DB.QueryRow(ctx, `UPDATE users SET display_name=coalesce($2,display_name),timezone=coalesce($3,timezone),locale=coalesce($4,locale),preferences=coalesce($5::jsonb,preferences),updated_at=now() WHERE id=$1 RETURNING display_name,timezone,locale,preferences`, u.ID, d.Name, d.Zone, d.Locale, pref).Scan(&u.Name, &u.Zone, &u.Locale, &u.Pref)
	return u, e
}
func (s *Store) Password(ctx context.Context, p Principal, current, next string) error {
	if !validPassword(next) || len(current) > 256 {
		return ErrInvalid
	}
	if e := s.acquire(ctx); e != nil {
		return e
	}
	defer func() { <-s.slots }()
	var old string
	if e := s.DB.QueryRow(ctx, "SELECT password_hash FROM users WHERE id=$1", p.User.ID).Scan(&old); e != nil {
		return e
	}
	if !matches(old, current) {
		return ErrInvalid
	}
	hash := hashPassword(next)
	tx, e := s.DB.Begin(ctx)
	if e != nil {
		return e
	}
	defer tx.Rollback(ctx)
	tag, e := tx.Exec(ctx, "UPDATE users SET password_hash=$1,updated_at=now() WHERE id=$2 AND password_hash=$3", hash, p.User.ID, old)
	if e != nil {
		return e
	}
	if tag.RowsAffected() != 1 {
		return ErrInvalid
	}
	_, e = tx.Exec(ctx, "UPDATE sessions SET revoked_at=now() WHERE user_id=$1 AND id<>$2 AND revoked_at IS NULL", p.User.ID, p.Session)
	if e != nil {
		return e
	}
	return tx.Commit(ctx)
}

type Session struct {
	ID      string    `json:"id"`
	Current bool      `json:"current"`
	UA      string    `json:"user_agent"`
	Created time.Time `json:"created_at"`
	Seen    time.Time `json:"last_seen_at"`
	Expires time.Time `json:"expires_at"`
}

func (s *Store) Sessions(ctx context.Context, p Principal) ([]Session, error) {
	rows, e := s.DB.Query(ctx, "SELECT id,id=$2,user_agent,created_at,last_seen_at,expires_at FROM sessions WHERE user_id=$1 AND revoked_at IS NULL AND expires_at>now() ORDER BY created_at DESC", p.User.ID, p.Session)
	if e != nil {
		return nil, e
	}
	return pgx.CollectRows(rows, pgx.RowToStructByPos[Session])
}
func (s *Store) Revoke(ctx context.Context, p Principal, id string) error {
	tag, e := s.DB.Exec(ctx, "UPDATE sessions SET revoked_at=now() WHERE user_id=$1 AND id::text=$2 AND revoked_at IS NULL", p.User.ID, id)
	if e == nil && tag.RowsAffected() == 0 {
		return pgx.ErrNoRows
	}
	return e
}
