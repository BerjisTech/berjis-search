package auth

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"

	coreauth "github.com/berjistech/berjis-ecosystem/shared/coreauth"
)

type Options struct {
	Env         string
	CoreAPIBase string
	HTTPClient  *http.Client
	Verifier    *coreauth.Verifier
}

type User struct {
	ID    string
	Email string
	Roles []string
}

func (u *User) mergeRoles(roles ...string) {
	if u == nil || len(roles) == 0 {
		return
	}
	seen := make(map[string]struct{}, len(u.Roles))
	for _, r := range u.Roles {
		seen[r] = struct{}{}
	}
	for _, role := range roles {
		val := strings.ToLower(strings.TrimSpace(role))
		if val == "" {
			continue
		}
		if _, ok := seen[val]; ok {
			continue
		}
		u.Roles = append(u.Roles, val)
		seen[val] = struct{}{}
	}
}

func (u User) HasRole(role string) bool {
	role = strings.ToLower(strings.TrimSpace(role))
	if role == "" {
		return false
	}
	for _, r := range u.Roles {
		if r == role {
			return true
		}
	}
	return false
}

func (u User) HasAnyRole(roles ...string) bool {
	for _, role := range roles {
		if u.HasRole(role) {
			return true
		}
	}
	return false
}

type contextKey string

const userKey contextKey = "search.user"

var errUnauthorized = errors.New("unauthorized")

func Middleware(opts Options) fiber.Handler {
	client := opts.HTTPClient
	if client == nil {
		client = &http.Client{Timeout: 5 * time.Second}
	}
	devEnv := strings.EqualFold(opts.Env, "development")

	return func(c *fiber.Ctx) error {
		authHeader := c.Get("Authorization")
		cookieHeader := c.Get("Cookie")
		token := tokenFromHeaders(authHeader, cookieHeader)

		var user *User

		if opts.Verifier != nil && token != "" {
			if claims, err := opts.Verifier.Verify(token); err == nil {
				user = &User{ID: claims.UUID, Email: claims.Email}
			} else if errors.Is(err, coreauth.ErrTokenInvalid) || errors.Is(err, coreauth.ErrTokenExpired) || errors.Is(err, coreauth.ErrTokenMissing) {
				return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"success": false, "message": "unauthorized"})
			}
		}

		var session *verifySession
		if user == nil || len(user.Roles) == 0 {
			if s, err := verifyViaAPI(client, opts.CoreAPIBase, authHeader, cookieHeader); err == nil && s.Valid {
				session = s
			}
		}

		if user == nil && devEnv {
			if id := strings.TrimSpace(c.Get("X-User-UUID")); id != "" {
				user = &User{ID: id, Email: c.Get("X-User-Email")}
			}
		}

		if session != nil {
			if user == nil {
				user = &User{ID: session.UUID, Email: session.Email}
			}
			if user.Email == "" {
				user.Email = session.Email
			}
			user.mergeRoles(session.Roles...)
			user.mergeRoles(session.PlatformRoles...)
			for _, arr := range session.AppRoles {
				user.mergeRoles(arr...)
			}
		}

		if user == nil || strings.TrimSpace(user.ID) == "" {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"success": false, "message": "login required"})
		}

		user.mergeRoles(strings.Split(c.Get("X-User-Roles"), ",")...)
		c.Locals(userKey, *user)
		return c.Next()
	}
}

type verifySession struct {
	Valid         bool
	UUID          string
	Email         string
	Roles         []string
	PlatformRoles []string
	AppRoles      map[string][]string
}

func verifyViaAPI(client *http.Client, base string, authHeader string, cookieHeader string) (*verifySession, error) {
	base = strings.TrimSpace(base)
	if base == "" {
		return nil, errors.New("core api base missing")
	}
	req, _ := http.NewRequest(http.MethodGet, strings.TrimRight(base, "/")+"/v1/auth/verify", nil)
	if authHeader != "" {
		req.Header.Set("Authorization", authHeader)
	}
	if cookieHeader != "" {
		req.Header.Set("Cookie", cookieHeader)
	}
	req.Header.Set("Accept", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, errors.New("unexpected status")
	}
	var payload struct {
		Data struct {
			Valid         bool                `json:"valid"`
			UUID          string              `json:"uuid"`
			UID           string              `json:"uid"`
			UserID        string              `json:"userId"`
			Email         string              `json:"email"`
			Roles         []string            `json:"roles"`
			PlatformRoles []string            `json:"platformRoles"`
			AppRoles      map[string][]string `json:"appRoles"`
		} `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return nil, err
	}
	data := payload.Data
	session := &verifySession{
		Valid:         data.Valid,
		UUID:          coalesceID(data.UUID, data.UID, data.UserID),
		Email:         data.Email,
		Roles:         lowerStrings(data.Roles),
		PlatformRoles: lowerStrings(data.PlatformRoles),
		AppRoles:      make(map[string][]string, len(data.AppRoles)),
	}
	for key, arr := range data.AppRoles {
		session.AppRoles[key] = lowerStrings(arr)
	}
	return session, nil
}

func coalesceID(values ...string) string {
	for _, v := range values {
		if strings.TrimSpace(v) != "" {
			return strings.TrimSpace(v)
		}
	}
	return ""
}

func lowerStrings(values []string) []string {
	out := make([]string, 0, len(values))
	for _, v := range values {
		if trimmed := strings.TrimSpace(v); trimmed != "" {
			out = append(out, strings.ToLower(trimmed))
		}
	}
	return out
}

func tokenFromHeaders(authHeader, cookieHeader string) string {
	authz := strings.TrimSpace(authHeader)
	if strings.HasPrefix(strings.ToLower(authz), "bearer ") {
		return strings.TrimSpace(authz[7:])
	}
	if authz != "" {
		return authz
	}
	return strings.TrimSpace(cookieHeader)
}

func UserFromCtx(c *fiber.Ctx) User {
	if v := c.Locals(userKey); v != nil {
		if u, ok := v.(User); ok {
			return u
		}
	}
	return User{}
}

func RequireRoles(c *fiber.Ctx, roles ...string) error {
	if len(roles) == 0 {
		return nil
	}
	user := UserFromCtx(c)
	for _, role := range roles {
		if user.HasRole(role) {
			return nil
		}
	}
	return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"success": false, "message": "forbidden"})
}

func RequireRolesMiddleware(roles ...string) fiber.Handler {
	target := make([]string, 0, len(roles))
	for _, role := range roles {
		if trimmed := strings.TrimSpace(role); trimmed != "" {
			target = append(target, strings.ToLower(trimmed))
		}
	}
	return func(c *fiber.Ctx) error {
		if err := RequireRoles(c, target...); err != nil {
			return err
		}
		return c.Next()
	}
}
