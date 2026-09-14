package main

import (
	"context"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"golang.org/x/crypto/pbkdf2"
)

const (
	authSuccessTTL = 60 * time.Second

	authFailureTTL = 2 * time.Second

	authCacheMaxEntries = 4096

	authPBKDF2Concurrency = 2
)

var errInvalidPBKDF2Hash = errors.New("invalid PBKDF2 hash")

type authUser struct {
	iterations int
	salt       []byte

	expected []byte
}

type authCacheKey struct {
	username string
	password [32]byte
}

type authCacheEntry struct {
	validUntil time.Time
	generation uint64
	ok         bool
}

type authInflight struct {
	done chan struct{}
	ok   bool
}

type AuthStore struct {
	pool *pgxpool.Pool

	users atomic.Value

	generation atomic.Uint64

	cache sync.Map // map[authCacheKey]authCacheEntry

	cacheCount atomic.Int64

	pbkdf2Sem chan struct{}

	inflightMu sync.Mutex
	inflight   map[authCacheKey]*authInflight
}

func (s *AuthStore) CheckCredentials(
	username string,
	password string,
) bool {
	users := s.users.Load().(map[string]authUser)

	user, exists := users[username]
	if !exists {
		debugf(
			"[AUTH] user not found username=%s",
			username,
		)
		return false
	}

	passwordDigest := sha256.Sum256([]byte(password))

	key := authCacheKey{
		username: username,
		password: passwordDigest,
	}

	generation := s.generation.Load()
	now := time.Now()

	if value, found := s.cache.Load(key); found {
		entry := value.(authCacheEntry)

		if entry.generation == generation &&
			now.Before(entry.validUntil) {
			return entry.ok
		}

		if _, deleted := s.cache.LoadAndDelete(key); deleted {
			s.cacheCount.Add(-1)
		}
	}

	s.inflightMu.Lock()

	if existing, found := s.inflight[key]; found {
		s.inflightMu.Unlock()

		<-existing.done

		return existing.ok
	}

	current := &authInflight{
		done: make(chan struct{}),
	}

	s.inflight[key] = current

	s.inflightMu.Unlock()

	ok := s.checkPBKDF2(user, password)

	ttl := authFailureTTL
	if ok {
		ttl = authSuccessTTL
	}

	entry := authCacheEntry{
		validUntil: time.Now().Add(ttl),
		generation: generation,
		ok:         ok,
	}

	s.storeCache(key, entry)

	s.inflightMu.Lock()

	current.ok = ok
	delete(s.inflight, key)
	close(current.done)

	s.inflightMu.Unlock()

	if !ok {
		debugf(
			"[AUTH] invalid credentials username=%s",
			username,
		)
	}

	return ok
}

func (s *AuthStore) checkPBKDF2(
	user authUser,
	password string,
) bool {
	// Limit CPU consumption.
	s.pbkdf2Sem <- struct{}{}

	dk := pbkdf2.Key(
		[]byte(password),
		user.salt,
		user.iterations,
		len(user.expected),
		sha256.New,
	)

	ok := subtle.ConstantTimeCompare(
		dk,
		user.expected,
	) == 1

	<-s.pbkdf2Sem

	return ok
}

func (s *AuthStore) storeCache(
	key authCacheKey,
	entry authCacheEntry,
) {
	if _, loaded := s.cache.LoadOrStore(key, entry); loaded {
		s.cache.Store(key, entry)
		return
	}

	count := s.cacheCount.Add(1)

	if count <= authCacheMaxEntries {
		return
	}

	s.cleanupCache()
}

func (s *AuthStore) cleanupCache() {
	now := time.Now()

	s.cache.Range(func(k, v any) bool {
		entry, ok := v.(authCacheEntry)
		if !ok {
			if _, deleted := s.cache.LoadAndDelete(k); deleted {
				s.cacheCount.Add(-1)
			}
			return true
		}

		if now.After(entry.validUntil) {
			if _, deleted := s.cache.LoadAndDelete(k); deleted {
				s.cacheCount.Add(-1)
			}
		}

		return true
	})

	for s.cacheCount.Load() > authCacheMaxEntries {
		removed := false

		s.cache.Range(func(k, _ any) bool {
			if _, deleted := s.cache.LoadAndDelete(k); deleted {
				s.cacheCount.Add(-1)
				removed = true
			}

			return false
		})

		if !removed {
			break
		}
	}
}

func NewAuthStore(
	ctx context.Context,
	dbURI string,
) (*AuthStore, error) {
	infof("[AUTH] initializing auth store")

	pool, err := pgxpool.New(ctx, dbURI)
	if err != nil {
		warnf(
			"[AUTH] failed to create database pool: %v",
			err,
		)
		return nil, err
	}

	store := &AuthStore{
		pool: pool,

		pbkdf2Sem: make(
			chan struct{},
			authPBKDF2Concurrency,
		),

		inflight: make(
			map[authCacheKey]*authInflight,
		),
	}

	if err := store.reload(ctx); err != nil {
		warnf(
			"[AUTH] initial users reload failed: %v",
			err,
		)

		pool.Close()

		return nil, err
	}

	infof("[AUTH] auth store initialized")

	return store, nil
}

func (s *AuthStore) reload(ctx context.Context) error {
	reloadCtx, cancel := context.WithTimeout(
		ctx,
		AuthReloadTimeout,
	)
	defer cancel()

	debugf("[AUTH] reloading users")

	rows, err := s.pool.Query(reloadCtx, `
		SELECT username, password
		FROM core_proxy
	`)
	if err != nil {
		warnf(
			"[AUTH] users query failed: %v",
			err,
		)
		return err
	}
	defer rows.Close()

	users := make(map[string]authUser)

	for rows.Next() {
		var username string
		var passwordHash string

		if err := rows.Scan(
			&username,
			&passwordHash,
		); err != nil {
			warnf(
				"[AUTH] users row scan failed: %v",
				err,
			)
			return err
		}

		user, err := parsePBKDF2(passwordHash)
		if err != nil {
			warnf(
				"[AUTH] invalid password hash username=%s: %v",
				username,
				err,
			)
			continue
		}

		users[username] = user
	}

	if err := rows.Err(); err != nil {
		warnf(
			"[AUTH] users rows iteration failed: %v",
			err,
		)
		return err
	}

	s.users.Store(users)

	s.generation.Add(1)

	infof(
		"[AUTH] users reloaded count=%d",
		len(users),
	)

	return nil
}

func parsePBKDF2(encoded string) (authUser, error) {
	parts := strings.SplitN(encoded, "$", 4)

	if len(parts) != 4 {
		return authUser{}, errInvalidPBKDF2Hash
	}

	if parts[0] != "pbkdf2_sha256" {
		return authUser{}, errInvalidPBKDF2Hash
	}

	iterations, err := strconv.Atoi(parts[1])
	if err != nil || iterations <= 0 {
		return authUser{}, errInvalidPBKDF2Hash
	}

	salt := []byte(parts[2])

	expected, err := base64.StdEncoding.DecodeString(parts[3])
	if err != nil {
		return authUser{}, errInvalidPBKDF2Hash
	}

	if len(expected) != 32 {
		return authUser{}, errInvalidPBKDF2Hash
	}

	return authUser{
		iterations: iterations,
		salt:       salt,
		expected:   expected,
	}, nil
}

func (s *AuthStore) StartSync(ctx context.Context) {
	infof(
		"[AUTH] starting user sync interval=%s",
		AuthSyncInterval,
	)

	ticker := time.NewTicker(AuthSyncInterval)

	go func() {
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				infof("[AUTH] user sync stopped")
				return

			case <-ticker.C:
				if err := s.reload(ctx); err != nil {
					warnf(
						"[AUTH] sync failed: %v",
						err,
					)
				}
			}
		}
	}()
}

func (s *AuthStore) CheckAuth(req *http.Request) bool {
	values := req.Header.Values("Proxy-Authorization")

	if len(values) != 1 {
		debugf(
			"[AUTH] invalid Proxy-Authorization header count=%d",
			len(values),
		)
		return false
	}

	scheme, encoded, ok := strings.Cut(
		values[0],
		" ",
	)
	if !ok || !strings.EqualFold(scheme, "Basic") {
		debugf("[AUTH] unsupported Proxy-Authorization scheme")
		return false
	}

	decoded, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		debugf("[AUTH] invalid Basic authorization encoding")
		return false
	}

	username, password, ok := strings.Cut(
		string(decoded),
		":",
	)
	if !ok {
		debugf("[AUTH] invalid Basic authorization format")
		return false
	}

	ok = s.CheckCredentials(
		username,
		password,
	)

	if ok {
		debugf(
			"[AUTH] authentication successful username=%s",
			username,
		)
	} else {
		debugf(
			"[AUTH] authentication failed username=%s",
			username,
		)
	}

	return ok
}

func (s *AuthStore) Close() {
	infof("[AUTH] closing auth store")
	s.pool.Close()
}
