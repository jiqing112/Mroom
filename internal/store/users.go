package store

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/md5"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"strings"
	"time"

	"golang.org/x/crypto/bcrypt"

	"mrome/internal/util"
)

var ErrInvalidCredentials = errors.New("invalid credentials")

func hashID(prefix string, parts ...string) string {
	return util.MD5Hex(append([]string{prefix}, parts...)...)
}

func randomHex(n int) string { return util.RandomHex(n) }

// cipherKey derives a stable AES key from the instance secret.
func (s *Store) cipherKey() []byte {
	sum := sha256.Sum256(append([]byte("mrome-password-vault:"), s.secret...))
	return sum[:]
}

// encryptPassword keeps a reversible copy of the password so that Subsonic
// token authentication (md5(password+salt)) can be verified server-side.
// The copy is AES-GCM encrypted with an instance-local key.
func (s *Store) encryptPassword(plain string) (string, error) {
	block, err := aes.NewCipher(s.cipherKey())
	if err != nil {
		return "", err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return "", err
	}
	out := gcm.Seal(nonce, nonce, []byte(plain), nil)
	return base64.StdEncoding.EncodeToString(out), nil
}

func (s *Store) decryptPassword(enc string) string {
	if enc == "" {
		return ""
	}
	raw, err := base64.StdEncoding.DecodeString(enc)
	if err != nil {
		return ""
	}
	block, err := aes.NewCipher(s.cipherKey())
	if err != nil {
		return ""
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil || len(raw) < gcm.NonceSize() {
		return ""
	}
	plain, err := gcm.Open(nil, raw[:gcm.NonceSize()], raw[gcm.NonceSize():], nil)
	if err != nil {
		return ""
	}
	return string(plain)
}

func (s *Store) CreateUser(username, password string, isAdmin bool) (*User, error) {
	username = strings.TrimSpace(username)
	if username == "" || len(password) < 3 {
		return nil, errors.New("用户名不能为空，密码至少 3 位")
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return nil, err
	}
	enc, err := s.encryptPassword(password)
	if err != nil {
		return nil, err
	}
	u := &User{
		ID:       hashID("user", strings.ToLower(username)),
		Username: username,
		IsAdmin:  isAdmin,
		APIKey:   randomHex(20),
		Created:  time.Now().Unix(),
	}
	_, err = s.db.Exec(`INSERT INTO users(id,username,password_hash,password_enc,is_admin,api_key,created_at)
		VALUES(?,?,?,?,?,?,?)`, u.ID, u.Username, string(hash), enc, boolInt(isAdmin), u.APIKey, u.Created)
	if err != nil {
		return nil, err
	}
	return u, nil
}

func boolInt(b bool) int {
	if b {
		return 1
	}
	return 0
}

func (s *Store) Authenticate(username, password string) (*User, error) {
	var u User
	var hash, enc string
	var admin int
	err := s.db.QueryRow(`SELECT id, username, password_hash, password_enc, is_admin, api_key, session_version, created_at
		FROM users WHERE username = ?`, username).Scan(&u.ID, &u.Username, &hash, &enc, &admin, &u.APIKey, &u.SessionVersion, &u.Created)
	if err != nil {
		return nil, ErrInvalidCredentials
	}
	if bcrypt.CompareHashAndPassword([]byte(hash), []byte(password)) != nil {
		return nil, ErrInvalidCredentials
	}
	u.IsAdmin = admin == 1
	return &u, nil
}

// CheckSubsonicToken verifies md5(password + salt).
func (s *Store) CheckSubsonicToken(username, token, salt string) (*User, error) {
	var u User
	var enc string
	var admin int
	err := s.db.QueryRow(`SELECT id, username, password_enc, is_admin, api_key, created_at
		FROM users WHERE username = ?`, username).Scan(&u.ID, &u.Username, &enc, &admin, &u.APIKey, &u.Created)
	if err != nil {
		return nil, ErrInvalidCredentials
	}
	plain := s.decryptPassword(enc)
	if plain == "" {
		return nil, ErrInvalidCredentials
	}
	sum := md5.Sum([]byte(plain + salt))
	if !strings.EqualFold(hex.EncodeToString(sum[:]), token) {
		return nil, ErrInvalidCredentials
	}
	u.IsAdmin = admin == 1
	return &u, nil
}

func (s *Store) UserByAPIKey(key string) (*User, error) {
	var u User
	var admin int
	err := s.db.QueryRow(`SELECT id, username, is_admin, api_key, created_at FROM users WHERE api_key=? AND api_key<>''`, key).
		Scan(&u.ID, &u.Username, &admin, &u.APIKey, &u.Created)
	if err != nil {
		return nil, ErrInvalidCredentials
	}
	u.IsAdmin = admin == 1
	return &u, nil
}

func (s *Store) GetUserByName(username string) (*User, error) {
	var u User
	var admin int
	err := s.db.QueryRow(`SELECT id, username, is_admin, api_key, created_at FROM users WHERE username=?`, username).
		Scan(&u.ID, &u.Username, &admin, &u.APIKey, &u.Created)
	if err != nil {
		return nil, err
	}
	u.IsAdmin = admin == 1
	return &u, nil
}

func (s *Store) GetUserByID(id string) (*User, error) {
	var u User
	var admin int
	err := s.db.QueryRow(`SELECT id, username, is_admin, api_key, session_version, created_at FROM users WHERE id=?`, id).
		Scan(&u.ID, &u.Username, &admin, &u.APIKey, &u.SessionVersion, &u.Created)
	if err != nil {
		return nil, err
	}
	u.IsAdmin = admin == 1
	return &u, nil
}

func (s *Store) ListUsers() ([]User, error) {
	rows, err := s.db.Query(`SELECT id, username, is_admin, api_key, created_at FROM users ORDER BY created_at`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []User{}
	for rows.Next() {
		var u User
		var admin int
		if err := rows.Scan(&u.ID, &u.Username, &admin, &u.APIKey, &u.Created); err != nil {
			return nil, err
		}
		u.IsAdmin = admin == 1
		out = append(out, u)
	}
	return out, rows.Err()
}

func (s *Store) UpdateUser(id, password string, isAdmin *bool) error {
	if isAdmin != nil && !*isAdmin {
		var current int
		var admins int
		_ = s.db.QueryRow(`SELECT is_admin FROM users WHERE id=?`, id).Scan(&current)
		_ = s.db.QueryRow(`SELECT COUNT(*) FROM users WHERE is_admin=1`).Scan(&admins)
		if current == 1 && admins <= 1 {
			return errors.New("不能取消唯一的管理员账号的管理员权限")
		}
	}
	if password != "" {
		hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
		if err != nil {
			return err
		}
		enc, err := s.encryptPassword(password)
		if err != nil {
			return err
		}
		// Bump the session version so cookies issued before the password change
		// stop authenticating immediately.
		if _, err := s.db.Exec(`UPDATE users SET password_hash=?, password_enc=?, session_version=session_version+1 WHERE id=?`,
			string(hash), enc, id); err != nil {
			return err
		}
	}
	if isAdmin != nil {
		if _, err := s.db.Exec(`UPDATE users SET is_admin=? WHERE id=?`, boolInt(*isAdmin), id); err != nil {
			return err
		}
	}
	return nil
}

func (s *Store) DeleteUser(id string) error {
	var admins int
	_ = s.db.QueryRow(`SELECT COUNT(*) FROM users WHERE is_admin=1`).Scan(&admins)
	var isAdmin int
	_ = s.db.QueryRow(`SELECT is_admin FROM users WHERE id=?`, id).Scan(&isAdmin)
	if isAdmin == 1 && admins <= 1 {
		return errors.New("不能删除唯一的管理员账号")
	}
	_, err := s.db.Exec(`DELETE FROM users WHERE id=?`, id)
	return err
}

func (s *Store) RegenerateAPIKey(id string) (string, error) {
	key := randomHex(20)
	_, err := s.db.Exec(`UPDATE users SET api_key=? WHERE id=?`, key, id)
	return key, err
}

func (s *Store) CountUsers() (int, error) {
	var n int
	err := s.db.QueryRow(`SELECT COUNT(*) FROM users`).Scan(&n)
	return n, err
}

func (s *Store) CountAdmins() (int, error) {
	var n int
	err := s.db.QueryRow(`SELECT COUNT(*) FROM users WHERE is_admin=1`).Scan(&n)
	return n, err
}

// EnsureGuest returns the shared account used by guest mode, creating it on
// first use. Its password is random and never exposed: guests are signed in by
// the server, not by submitting the login form.
func (s *Store) EnsureGuest() (*User, error) {
	if id := s.GetSetting("guest_user_id", ""); id != "" {
		if u, err := s.GetUserByID(id); err == nil {
			if u.IsAdmin {
				return nil, errors.New("访客账号不能是管理员")
			}
			return u, nil
		}
	}
	if u, err := s.GetUserByName("访客"); err == nil {
		if u.IsAdmin {
			return nil, errors.New("访客账号不能是管理员")
		}
		if full, ferr := s.GetUserByID(u.ID); ferr == nil {
			u = full
		}
		_ = s.SetSetting("guest_user_id", u.ID)
		return u, nil
	}
	u, err := s.CreateUser("访客", randomHex(24), false)
	if err != nil {
		return nil, err
	}
	_ = s.SetSetting("guest_user_id", u.ID)
	return u, nil
}
