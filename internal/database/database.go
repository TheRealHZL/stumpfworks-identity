package database

import (
	"context"
	"database/sql"
	"fmt"
	_ "modernc.org/sqlite"
	"os"
	"path/filepath"
	"time"
)

const migration = `CREATE TABLE IF NOT EXISTS users (id INTEGER PRIMARY KEY AUTOINCREMENT, username TEXT NOT NULL UNIQUE, display_name TEXT NOT NULL, directory_dn TEXT NOT NULL DEFAULT '', pin_hash TEXT NOT NULL DEFAULT '', created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP, updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP);
CREATE TABLE IF NOT EXISTS badges (id INTEGER PRIMARY KEY AUTOINCREMENT, badge_code TEXT NOT NULL UNIQUE, user_id INTEGER NOT NULL REFERENCES users(id), token_hash TEXT NOT NULL, enabled BOOLEAN NOT NULL DEFAULT 1, description TEXT NOT NULL DEFAULT '', issued_by TEXT NOT NULL DEFAULT '', created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP, updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP, last_used_at DATETIME, revoked_at DATETIME);
CREATE TABLE IF NOT EXISTS audit_log (id INTEGER PRIMARY KEY AUTOINCREMENT, event_type TEXT NOT NULL, badge_id TEXT NOT NULL DEFAULT '', username TEXT NOT NULL DEFAULT '', client_id TEXT NOT NULL DEFAULT '', success BOOLEAN NOT NULL, ip_address TEXT NOT NULL DEFAULT '', timestamp DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP, details TEXT NOT NULL DEFAULT '');
CREATE INDEX IF NOT EXISTS idx_badges_code ON badges(badge_code); CREATE INDEX IF NOT EXISTS idx_audit_timestamp ON audit_log(timestamp);`

type Store struct{ DB *sql.DB }
type User struct {
	ID          int64     `json:"id"`
	Username    string    `json:"username"`
	DisplayName string    `json:"display_name"`
	DirectoryDN string    `json:"directory_dn"`
	PINHash     string    `json:"-"`
	PINEnabled  bool      `json:"pin_enabled"`
	CreatedAt   time.Time `json:"created_at"`
}
type Badge struct {
	ID          int64        `json:"id"`
	BadgeCode   string       `json:"badge_code"`
	UserID      int64        `json:"user_id"`
	Username    string       `json:"username"`
	DisplayName string       `json:"display_name"`
	PINHash     string       `json:"-"`
	TokenHash   string       `json:"-"`
	Enabled     bool         `json:"enabled"`
	Description string       `json:"description"`
	CreatedAt   time.Time    `json:"created_at"`
	LastUsedAt  sql.NullTime `json:"-"`
	RevokedAt   sql.NullTime `json:"-"`
}
type Audit struct {
	ID                                     int64 `json:"id"`
	EventType, BadgeID, Username, ClientID string
	Success                                bool
	IPAddress                              string
	Timestamp                              time.Time
	Details                                string
}

func Open(path string) (*Store, error) {
	if path != ":memory:" {
		if err := os.MkdirAll(filepath.Dir(path), 0750); err != nil {
			return nil, err
		}
	}
	db, err := sql.Open("sqlite", path+"?_pragma=foreign_keys(1)&_pragma=busy_timeout(5000)")
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(1)
	if _, err = db.Exec(migration); err != nil {
		db.Close()
		return nil, err
	}
	var pinColumns int
	if err = db.QueryRow(`SELECT count(*) FROM pragma_table_info('users') WHERE name='pin_hash'`).Scan(&pinColumns); err != nil {
		db.Close()
		return nil, err
	}
	if pinColumns == 0 {
		if _, err = db.Exec(`ALTER TABLE users ADD COLUMN pin_hash TEXT NOT NULL DEFAULT ''`); err != nil {
			db.Close()
			return nil, err
		}
	}
	return &Store{db}, nil
}
func (s *Store) Close() error { return s.DB.Close() }
func (s *Store) CreateUser(ctx context.Context, u, d, dn string) (User, error) {
	r, err := s.DB.ExecContext(ctx, `INSERT INTO users(username,display_name,directory_dn) VALUES(?,?,?)`, u, d, dn)
	if err != nil {
		return User{}, err
	}
	id, _ := r.LastInsertId()
	return s.GetUser(ctx, id)
}
func (s *Store) GetUser(ctx context.Context, id int64) (u User, err error) {
	err = s.DB.QueryRowContext(ctx, `SELECT id,username,display_name,directory_dn,pin_hash,pin_hash!='',created_at FROM users WHERE id=?`, id).Scan(&u.ID, &u.Username, &u.DisplayName, &u.DirectoryDN, &u.PINHash, &u.PINEnabled, &u.CreatedAt)
	return
}
func (s *Store) UserByUsername(ctx context.Context, username string) (u User, err error) {
	err = s.DB.QueryRowContext(ctx, `SELECT id,username,display_name,directory_dn,pin_hash,pin_hash!='',created_at FROM users WHERE lower(username)=lower(?)`, username).Scan(&u.ID, &u.Username, &u.DisplayName, &u.DirectoryDN, &u.PINHash, &u.PINEnabled, &u.CreatedAt)
	return
}
func (s *Store) Users(ctx context.Context) ([]User, error) {
	rows, e := s.DB.QueryContext(ctx, `SELECT id,username,display_name,directory_dn,pin_hash,pin_hash!='',created_at FROM users ORDER BY username`)
	if e != nil {
		return nil, e
	}
	defer rows.Close()
	out := []User{}
	for rows.Next() {
		var u User
		if e = rows.Scan(&u.ID, &u.Username, &u.DisplayName, &u.DirectoryDN, &u.PINHash, &u.PINEnabled, &u.CreatedAt); e != nil {
			return nil, e
		}
		out = append(out, u)
	}
	return out, rows.Err()
}
func (s *Store) SetUserPIN(ctx context.Context, id int64, hash string) error {
	r, e := s.DB.ExecContext(ctx, `UPDATE users SET pin_hash=?,updated_at=CURRENT_TIMESTAMP WHERE id=?`, hash, id)
	if e != nil {
		return e
	}
	n, _ := r.RowsAffected()
	if n != 1 {
		return sql.ErrNoRows
	}
	return nil
}
func (s *Store) nextCode(ctx context.Context, tx *sql.Tx) (string, error) {
	var n int64
	if e := tx.QueryRowContext(ctx, `SELECT COALESCE(MAX(id),0)+1 FROM badges`).Scan(&n); e != nil {
		return "", e
	}
	return fmt.Sprintf("SW-%04d", n), nil
}
func (s *Store) CreateBadge(ctx context.Context, user int64, hash, desc string) (Badge, error) {
	tx, e := s.DB.BeginTx(ctx, nil)
	if e != nil {
		return Badge{}, e
	}
	defer tx.Rollback()
	code, e := s.nextCode(ctx, tx)
	if e != nil {
		return Badge{}, e
	}
	r, e := tx.ExecContext(ctx, `INSERT INTO badges(badge_code,user_id,token_hash,description) VALUES(?,?,?,?)`, code, user, hash, desc)
	if e != nil {
		return Badge{}, e
	}
	id, _ := r.LastInsertId()
	if e = tx.Commit(); e != nil {
		return Badge{}, e
	}
	return s.GetBadge(ctx, id)
}
func (s *Store) GetBadge(ctx context.Context, id int64) (b Badge, err error) {
	err = s.DB.QueryRowContext(ctx, `SELECT b.id,b.badge_code,b.user_id,u.username,u.display_name,u.pin_hash,b.token_hash,b.enabled,b.description,b.created_at,b.last_used_at,b.revoked_at FROM badges b JOIN users u ON u.id=b.user_id WHERE b.id=?`, id).Scan(&b.ID, &b.BadgeCode, &b.UserID, &b.Username, &b.DisplayName, &b.PINHash, &b.TokenHash, &b.Enabled, &b.Description, &b.CreatedAt, &b.LastUsedAt, &b.RevokedAt)
	return
}
func (s *Store) BadgeByCode(ctx context.Context, c string) (b Badge, err error) {
	err = s.DB.QueryRowContext(ctx, `SELECT b.id,b.badge_code,b.user_id,u.username,u.display_name,u.pin_hash,b.token_hash,b.enabled,b.description,b.created_at,b.last_used_at,b.revoked_at FROM badges b JOIN users u ON u.id=b.user_id WHERE b.badge_code=?`, c).Scan(&b.ID, &b.BadgeCode, &b.UserID, &b.Username, &b.DisplayName, &b.PINHash, &b.TokenHash, &b.Enabled, &b.Description, &b.CreatedAt, &b.LastUsedAt, &b.RevokedAt)
	return
}
func (s *Store) Badges(ctx context.Context) ([]Badge, error) {
	rows, e := s.DB.QueryContext(ctx, `SELECT b.id,b.badge_code,b.user_id,u.username,u.display_name,u.pin_hash,b.token_hash,b.enabled,b.description,b.created_at,b.last_used_at,b.revoked_at FROM badges b JOIN users u ON u.id=b.user_id ORDER BY b.id DESC`)
	if e != nil {
		return nil, e
	}
	defer rows.Close()
	out := []Badge{}
	for rows.Next() {
		var b Badge
		if e = rows.Scan(&b.ID, &b.BadgeCode, &b.UserID, &b.Username, &b.DisplayName, &b.PINHash, &b.TokenHash, &b.Enabled, &b.Description, &b.CreatedAt, &b.LastUsedAt, &b.RevokedAt); e != nil {
			return nil, e
		}
		out = append(out, b)
	}
	return out, rows.Err()
}
func (s *Store) Revoke(ctx context.Context, id int64) error {
	_, e := s.DB.ExecContext(ctx, `UPDATE badges SET enabled=0,revoked_at=CURRENT_TIMESTAMP,updated_at=CURRENT_TIMESTAMP WHERE id=?`, id)
	return e
}
func (s *Store) Used(ctx context.Context, id int64) error {
	_, e := s.DB.ExecContext(ctx, `UPDATE badges SET last_used_at=CURRENT_TIMESTAMP WHERE id=?`, id)
	return e
}
func (s *Store) Audit(ctx context.Context, event, bid, user, client string, success bool, ip, details string) {
	_, _ = s.DB.ExecContext(ctx, `INSERT INTO audit_log(event_type,badge_id,username,client_id,success,ip_address,details) VALUES(?,?,?,?,?,?,?)`, event, bid, user, client, success, ip, details)
}
func (s *Store) Audits(ctx context.Context) ([]Audit, error) {
	rows, e := s.DB.QueryContext(ctx, `SELECT id,event_type,badge_id,username,client_id,success,ip_address,timestamp,details FROM audit_log ORDER BY id DESC LIMIT 200`)
	if e != nil {
		return nil, e
	}
	defer rows.Close()
	out := []Audit{}
	for rows.Next() {
		var a Audit
		if e = rows.Scan(&a.ID, &a.EventType, &a.BadgeID, &a.Username, &a.ClientID, &a.Success, &a.IPAddress, &a.Timestamp, &a.Details); e != nil {
			return nil, e
		}
		out = append(out, a)
	}
	return out, rows.Err()
}
func (s *Store) Stats(ctx context.Context) map[string]int {
	out := map[string]int{}
	for k, q := range map[string]string{"users": "SELECT count(*) FROM users", "badges": "SELECT count(*) FROM badges", "active": "SELECT count(*) FROM badges WHERE enabled=1", "revoked": "SELECT count(*) FROM badges WHERE enabled=0", "today": "SELECT count(*) FROM audit_log WHERE event_type='auth_success' AND date(timestamp)=date('now')", "failed": "SELECT count(*) FROM audit_log WHERE event_type='auth_failed' AND date(timestamp)=date('now')"} {
		var n int
		_ = s.DB.QueryRowContext(ctx, q).Scan(&n)
		out[k] = n
	}
	return out
}
