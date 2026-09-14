package main

import (
	"encoding/base64"
	"net/http"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"golang.org/x/crypto/pbkdf2"
	"crypto/sha256"
)

const testPasswordHash = "pbkdf2_sha256$1500000$1y5fQZYtxH493JrkweFCUC$C0rF6PexrwVsIbLMtTR2ldJ5AHP0B5KFRjAULlRhIxo="

func newTestAuthStore() *AuthStore {
	user, err := parsePBKDF2(testPasswordHash)
	if err != nil {
		panic(err)
	}

	store := &AuthStore{
		pbkdf2Sem: make(chan struct{}, authPBKDF2Concurrency),
		inflight:  make(map[authCacheKey]*authInflight),
	}

	store.users.Store(map[string]authUser{
		"alice": user,
	})

	return store
}

func TestParsePBKDF2(t *testing.T) {
	tests := []struct {
		name    string
		encoded string
		wantErr bool
	}{
		{
			name:    "valid hash",
			encoded: testPasswordHash,
			wantErr: false,
		},
		{
			name:    "invalid format",
			encoded: "invalid",
			wantErr: true,
		},
		{
			name:    "wrong algorithm",
			encoded: "bcrypt$1500000$salt$hash",
			wantErr: true,
		},
		{
			name:    "invalid iterations",
			encoded: "pbkdf2_sha256$abc$salt$C0rF6PexrwVsIbLMtTR2ldJ5AHP0B5KFRjAULlRhIxo=",
			wantErr: true,
		},
		{
			name:    "zero iterations",
			encoded: "pbkdf2_sha256$0$salt$C0rF6PexrwVsIbLMtTR2ldJ5AHP0B5KFRjAULlRhIxo=",
			wantErr: true,
		},
		{
			name:    "invalid base64",
			encoded: "pbkdf2_sha256$1500000$salt$!!!",
			wantErr: true,
		},
		{
			name:    "wrong derived key length",
			encoded: "pbkdf2_sha256$1500000$salt$YWJj",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			user, err := parsePBKDF2(tt.encoded)

			if tt.wantErr {
				if err == nil {
					t.Fatalf("parsePBKDF2() error = nil, want error")
				}

				return
			}

			if err != nil {
				t.Fatalf("parsePBKDF2() unexpected error: %v", err)
			}

			if user.iterations != 1500000 {
				t.Errorf(
					"iterations = %d, want %d",
					user.iterations,
					1500000,
				)
			}

			if len(user.salt) == 0 {
				t.Error("salt is empty")
			}

			if len(user.expected) != 32 {
				t.Errorf(
					"expected length = %d, want 32",
					len(user.expected),
				)
			}
		})
	}
}

func TestCheckPBKDF2(t *testing.T) {
	user, err := parsePBKDF2(testPasswordHash)
	if err != nil {
		t.Fatal(err)
	}

	store := &AuthStore{
		pbkdf2Sem: make(chan struct{}, authPBKDF2Concurrency),
	}

	tests := []struct {
		name     string
		password string
		want     bool
	}{
		{
			name:     "correct password",
			password: "secret123",
			want:     true,
		},
		{
			name:     "wrong password",
			password: "wrong-password",
			want:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := store.checkPBKDF2(user, tt.password)

			if got != tt.want {
				t.Errorf(
					"checkPBKDF2() = %v, want %v",
					got,
					tt.want,
				)
			}
		})
	}
}

func TestCheckCredentials(t *testing.T) {
	store := newTestAuthStore()

	tests := []struct {
		name     string
		username string
		password string
		want     bool
	}{
		{
			name:     "correct credentials",
			username: "alice",
			password: "secret123",
			want:     true,
		},
		{
			name:     "wrong password",
			username: "alice",
			password: "wrong-password",
			want:     false,
		},
		{
			name:     "unknown user",
			username: "bob",
			password: "secret123",
			want:     false,
		},
		{
			name:     "empty username",
			username: "",
			password: "secret123",
			want:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := store.CheckCredentials(
				tt.username,
				tt.password,
			)

			if got != tt.want {
				t.Errorf(
					"CheckCredentials() = %v, want %v",
					got,
					tt.want,
				)
			}
		})
	}
}

func TestAuthStoreCheckAuth(t *testing.T) {
	store := newTestAuthStore()

	tests := []struct {
		name string
		auth string
		want bool
	}{
		{
			name: "correct credentials",
			auth: basicAuth("alice", "secret123"),
			want: true,
		},
		{
			name: "wrong password",
			auth: basicAuth("alice", "wrong-password"),
			want: false,
		},
		{
			name: "unknown user",
			auth: basicAuth("bob", "secret123"),
			want: false,
		},
		{
			name: "missing authorization header",
			auth: "",
			want: false,
		},
		{
			name: "wrong authentication scheme",
			auth: "Bearer abc123",
			want: false,
		},
		{
			name: "invalid base64",
			auth: "Basic !!!invalid!!!",
			want: false,
		},
		{
			name: "credentials without colon",
			auth: "Basic " + base64.StdEncoding.EncodeToString(
				[]byte("alice"),
			),
			want: false,
		},
		{
			name: "empty credentials",
			auth: basicAuth("", ""),
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req, err := http.NewRequest(
				http.MethodConnect,
				"http://example.com",
				nil,
			)
			if err != nil {
				t.Fatal(err)
			}

			if tt.auth != "" {
				req.Header.Set(
					"Proxy-Authorization",
					tt.auth,
				)
			}

			got := store.CheckAuth(req)

			if got != tt.want {
				t.Errorf(
					"CheckAuth() = %v, want %v",
					got,
					tt.want,
				)
			}
		})
	}
}

func TestCheckAuthCaseInsensitiveScheme(t *testing.T) {
	store := newTestAuthStore()

	req, err := http.NewRequest(
		http.MethodConnect,
		"http://example.com",
		nil,
	)
	if err != nil {
		t.Fatal(err)
	}

	credentials := base64.StdEncoding.EncodeToString(
		[]byte("alice:secret123"),
	)

	req.Header.Set(
		"Proxy-Authorization",
		"bAsIc "+credentials,
	)

	if !store.CheckAuth(req) {
		t.Error("CheckAuth() = false, want true")
	}
}

func TestCheckAuthMultipleHeaders(t *testing.T) {
	store := newTestAuthStore()

	req, err := http.NewRequest(
		http.MethodConnect,
		"http://example.com",
		nil,
	)
	if err != nil {
		t.Fatal(err)
	}

	req.Header.Add(
		"Proxy-Authorization",
		basicAuth("alice", "secret123"),
	)

	req.Header.Add(
		"Proxy-Authorization",
		basicAuth("alice", "secret123"),
	)

	if store.CheckAuth(req) {
		t.Error("CheckAuth() = true, want false")
	}
}

func TestAuthCacheSuccess(t *testing.T) {
	store := newTestAuthStore()

	username := "alice"
	password := "secret123"

	if !store.CheckCredentials(username, password) {
		t.Fatal("first CheckCredentials() = false, want true")
	}

	if store.cacheCount.Load() != 1 {
		t.Fatalf(
			"cacheCount = %d, want 1",
			store.cacheCount.Load(),
		)
	}

	passwordDigest := sha256.Sum256([]byte(password))

	key := authCacheKey{
		username: username,
		password: passwordDigest,
	}

	value, ok := store.cache.Load(key)
	if !ok {
		t.Fatal("expected credentials to be stored in cache")
	}

	entry := value.(authCacheEntry)

	if !entry.ok {
		t.Error("cached result = false, want true")
	}

	if entry.generation != store.generation.Load() {
		t.Errorf(
			"cache generation = %d, want %d",
			entry.generation,
			store.generation.Load(),
		)
	}

	if time.Until(entry.validUntil) <= 0 {
		t.Error("cached entry is already expired")
	}
}

func TestAuthCacheFailure(t *testing.T) {
	store := newTestAuthStore()

	username := "alice"
	password := "wrong-password"

	if store.CheckCredentials(username, password) {
		t.Fatal("CheckCredentials() = true, want false")
	}

	if store.cacheCount.Load() != 1 {
		t.Fatalf(
			"cacheCount = %d, want 1",
			store.cacheCount.Load(),
		)
	}

	passwordDigest := sha256.Sum256([]byte(password))

	key := authCacheKey{
		username: username,
		password: passwordDigest,
	}

	value, ok := store.cache.Load(key)
	if !ok {
		t.Fatal("expected failed credentials to be stored in cache")
	}

	entry := value.(authCacheEntry)

	if entry.ok {
		t.Error("cached result = true, want false")
	}

	if time.Until(entry.validUntil) <= 0 {
		t.Error("cached entry is already expired")
	}
}

func TestAuthCacheHit(t *testing.T) {
	store := newTestAuthStore()

	username := "alice"
	password := "secret123"

	// First call performs PBKDF2 and populates the cache.
	if !store.CheckCredentials(username, password) {
		t.Fatal("first CheckCredentials() = false, want true")
	}

	passwordDigest := sha256.Sum256([]byte(password))

	key := authCacheKey{
		username: username,
		password: passwordDigest,
	}

	// Replace the cache entry with an already-expired entry.
	// This lets us verify the cache hit without sleeping for 60 seconds.
	entry, ok := store.cache.Load(key)
	if !ok {
		t.Fatal("expected cache entry")
	}

	cached := entry.(authCacheEntry)

	// Make a fresh valid entry first.
	store.cache.Store(key, authCacheEntry{
		validUntil: time.Now().Add(time.Minute),
		generation: cached.generation,
		ok:         true,
	})

	// Calling again must use the cache.
	if !store.CheckCredentials(username, password) {
		t.Fatal("second CheckCredentials() = false, want true")
	}

	value, ok := store.cache.Load(key)
	if !ok {
		t.Fatal("expected cache entry after cache hit")
	}

	result := value.(authCacheEntry)

	if !result.ok {
		t.Error("cached result = false, want true")
	}
}

func TestAuthCacheExpiration(t *testing.T) {
	store := newTestAuthStore()

	username := "alice"
	password := "secret123"

	if !store.CheckCredentials(username, password) {
		t.Fatal("first CheckCredentials() = false, want true")
	}

	passwordDigest := sha256.Sum256([]byte(password))

	key := authCacheKey{
		username: username,
		password: passwordDigest,
	}

	value, ok := store.cache.Load(key)
	if !ok {
		t.Fatal("expected cache entry")
	}

	entry := value.(authCacheEntry)

	// Force expiration without sleeping 60 seconds.
	store.cache.Store(key, authCacheEntry{
		validUntil: time.Now().Add(-time.Second),
		generation: entry.generation,
		ok:         entry.ok,
	})

	// The next call must detect expiration, remove the old entry,
	// execute PBKDF2 again, and create a fresh cache entry.
	if !store.CheckCredentials(username, password) {
		t.Fatal("CheckCredentials() = false, want true")
	}

	value, ok = store.cache.Load(key)
	if !ok {
		t.Fatal("expected fresh cache entry after expiration")
	}

	newEntry := value.(authCacheEntry)

	if !time.Now().Before(newEntry.validUntil) {
		t.Error("new cache entry is already expired")
	}

	if newEntry.generation != store.generation.Load() {
		t.Errorf(
			"generation = %d, want %d",
			newEntry.generation,
			store.generation.Load(),
		)
	}
}

func TestAuthCacheGenerationInvalidation(t *testing.T) {
	store := newTestAuthStore()

	username := "alice"
	password := "secret123"

	if !store.CheckCredentials(username, password) {
		t.Fatal("first CheckCredentials() = false, want true")
	}

	passwordDigest := sha256.Sum256([]byte(password))

	key := authCacheKey{
		username: username,
		password: passwordDigest,
	}

	value, ok := store.cache.Load(key)
	if !ok {
		t.Fatal("expected cache entry")
	}

	entry := value.(authCacheEntry)

	// Entry itself is still valid, but generation changes.
	store.generation.Add(1)

	if !store.CheckCredentials(username, password) {
		t.Fatal("CheckCredentials() = false, want true")
	}

	value, ok = store.cache.Load(key)
	if !ok {
		t.Fatal("expected cache entry after generation change")
	}

	newEntry := value.(authCacheEntry)

	if newEntry.generation != store.generation.Load() {
		t.Errorf(
			"cache generation = %d, want %d",
			newEntry.generation,
			store.generation.Load(),
		)
	}

	if newEntry.generation == entry.generation {
		t.Error("cache entry was not refreshed after generation change")
	}
}

func TestAuthCacheSeparatePasswords(t *testing.T) {
	store := newTestAuthStore()

	if !store.CheckCredentials("alice", "secret123") {
		t.Fatal("correct credentials should succeed")
	}

	if store.CheckCredentials("alice", "wrong-password") {
		t.Fatal("wrong password should fail")
	}

	if store.cacheCount.Load() != 2 {
		t.Fatalf(
			"cacheCount = %d, want 2",
			store.cacheCount.Load(),
		)
	}
}

func TestAuthCacheSeparateUsers(t *testing.T) {
	user, err := parsePBKDF2(testPasswordHash)
	if err != nil {
		t.Fatal(err)
	}

	store := &AuthStore{
		pbkdf2Sem: make(chan struct{}, authPBKDF2Concurrency),
		inflight:  make(map[authCacheKey]*authInflight),
	}

	store.users.Store(map[string]authUser{
		"alice": user,
		"bob":   user,
	})

	if !store.CheckCredentials("alice", "secret123") {
		t.Fatal("alice authentication failed")
	}

	if !store.CheckCredentials("bob", "secret123") {
		t.Fatal("bob authentication failed")
	}

	if store.cacheCount.Load() != 2 {
		t.Fatalf(
			"cacheCount = %d, want 2",
			store.cacheCount.Load(),
		)
	}
}

func TestAuthInflight(t *testing.T) {
	store := newTestAuthStore()

	const goroutines = 10

	var wg sync.WaitGroup
	wg.Add(goroutines)

	var success atomic.Int64

	for i := 0; i < goroutines; i++ {
		go func() {
			defer wg.Done()

			if store.CheckCredentials(
				"alice",
				"secret123",
			) {
				success.Add(1)
			}
		}()
	}

	wg.Wait()

	if success.Load() != goroutines {
		t.Fatalf(
			"successful authentications = %d, want %d",
			success.Load(),
			goroutines,
		)
	}

	if store.cacheCount.Load() != 1 {
		t.Fatalf(
			"cacheCount = %d, want 1",
			store.cacheCount.Load(),
		)
	}

	if len(store.inflight) != 0 {
		t.Fatalf(
			"inflight count = %d, want 0",
			len(store.inflight),
		)
	}
}

func TestAuthInflightFailure(t *testing.T) {
	store := newTestAuthStore()

	const goroutines = 10

	var wg sync.WaitGroup
	wg.Add(goroutines)

	var failures atomic.Int64

	for i := 0; i < goroutines; i++ {
		go func() {
			defer wg.Done()

			if !store.CheckCredentials(
				"alice",
				"wrong-password",
			) {
				failures.Add(1)
			}
		}()
	}

	wg.Wait()

	if failures.Load() != goroutines {
		t.Fatalf(
			"failed authentications = %d, want %d",
			failures.Load(),
			goroutines,
		)
	}

	if store.cacheCount.Load() != 1 {
		t.Fatalf(
			"cacheCount = %d, want 1",
			store.cacheCount.Load(),
		)
	}

	if len(store.inflight) != 0 {
		t.Fatalf(
			"inflight count = %d, want 0",
			len(store.inflight),
		)
	}
}

func TestStoreCache(t *testing.T) {
	store := newTestAuthStore()

	key := authCacheKey{
		username: "alice",
		password: sha256.Sum256([]byte("secret123")),
	}

	store.storeCache(
		key,
		authCacheEntry{
			validUntil: time.Now().Add(time.Minute),
			generation: store.generation.Load(),
			ok:         true,
		},
	)

	if store.cacheCount.Load() != 1 {
		t.Fatalf(
			"cacheCount = %d, want 1",
			store.cacheCount.Load(),
		)
	}

	// Replacing an existing key must not increment cacheCount.
	store.storeCache(
		key,
		authCacheEntry{
			validUntil: time.Now().Add(time.Minute),
			generation: store.generation.Load(),
			ok:         false,
		},
	)

	if store.cacheCount.Load() != 1 {
		t.Fatalf(
			"cacheCount after replacement = %d, want 1",
			store.cacheCount.Load(),
		)
	}

	value, ok := store.cache.Load(key)
	if !ok {
		t.Fatal("expected cache entry")
	}

	entry := value.(authCacheEntry)

	if entry.ok {
		t.Error("cache entry = true, want false")
	}
}

func TestCleanupCache(t *testing.T) {
	store := newTestAuthStore()

	key1 := authCacheKey{
		username: "alice",
		password: sha256.Sum256([]byte("password1")),
	}

	key2 := authCacheKey{
		username: "alice",
		password: sha256.Sum256([]byte("password2")),
	}

	store.storeCache(
		key1,
		authCacheEntry{
			validUntil: time.Now().Add(-time.Minute),
			generation: store.generation.Load(),
			ok:         true,
		},
	)

	store.storeCache(
		key2,
		authCacheEntry{
			validUntil: time.Now().Add(time.Minute),
			generation: store.generation.Load(),
			ok:         true,
		},
	)

	store.cleanupCache()

	if _, ok := store.cache.Load(key1); ok {
		t.Error("expired cache entry was not removed")
	}

	if _, ok := store.cache.Load(key2); !ok {
		t.Error("valid cache entry was removed")
	}

	if store.cacheCount.Load() != 1 {
		t.Fatalf(
			"cacheCount = %d, want 1",
			store.cacheCount.Load(),
		)
	}
}

func TestAuthCacheFailureTTL(t *testing.T) {
	store := newTestAuthStore()

	if store.CheckCredentials(
		"alice",
		"wrong-password",
	) {
		t.Fatal("expected authentication failure")
	}

	key := authCacheKey{
		username: "alice",
		password: sha256.Sum256([]byte("wrong-password")),
	}

	value, ok := store.cache.Load(key)
	if !ok {
		t.Fatal("expected failed authentication to be cached")
	}

	entry := value.(authCacheEntry)

	remaining := time.Until(entry.validUntil)

	if remaining <= 0 {
		t.Fatal("failure cache entry is already expired")
	}

	if remaining > authFailureTTL {
		t.Fatalf(
			"failure cache TTL = %s, want <= %s",
			remaining,
			authFailureTTL,
		)
	}
}

func TestAuthCacheSuccessTTL(t *testing.T) {
	store := newTestAuthStore()

	if !store.CheckCredentials(
		"alice",
		"secret123",
	) {
		t.Fatal("expected authentication success")
	}

	key := authCacheKey{
		username: "alice",
		password: sha256.Sum256([]byte("secret123")),
	}

	value, ok := store.cache.Load(key)
	if !ok {
		t.Fatal("expected successful authentication to be cached")
	}

	entry := value.(authCacheEntry)

	remaining := time.Until(entry.validUntil)

	if remaining <= 0 {
		t.Fatal("success cache entry is already expired")
	}

	if remaining > authSuccessTTL {
		t.Fatalf(
			"success cache TTL = %s, want <= %s",
			remaining,
			authSuccessTTL,
		)
	}
}

func TestParsePBKDF2MatchesExpectedHash(t *testing.T) {
	user, err := parsePBKDF2(testPasswordHash)
	if err != nil {
		t.Fatal(err)
	}

	got := pbkdf2.Key(
		[]byte("secret123"),
		user.salt,
		user.iterations,
		len(user.expected),
		sha256.New,
	)

	if string(got) != string(user.expected) {
		t.Fatal("PBKDF2 result does not match stored hash")
	}
}

func basicAuth(username, password string) string {
	credentials := username + ":" + password

	return "Basic " + base64.StdEncoding.EncodeToString(
		[]byte(credentials),
	)
}
