package auth

import (
	"crypto/rand"
	"encoding/hex"

	"golang.org/x/crypto/bcrypt"
)

// Credential is one client allowed to exchange a secret for a token. Only the
// bcrypt hash is ever held in memory.
type Credential struct {
	ID         string
	SecretHash []byte
}

// Store authenticates clients.
type Store struct {
	hashes map[string][]byte
	// decoy is compared against when the client id is unknown, so a wrong id
	// costs the same time as a wrong secret and cannot be enumerated.
	decoy []byte
}

// NewStore indexes the configured credentials.
func NewStore(credentials []Credential) *Store {
	store := &Store{hashes: make(map[string][]byte, len(credentials))}
	for _, credential := range credentials {
		store.hashes[credential.ID] = credential.SecretHash
	}

	// A hash of an unguessable value: comparing against it always fails, but it
	// burns the same bcrypt work as a real comparison.
	filler := make([]byte, 32)
	if _, err := rand.Read(filler); err == nil {
		if hash, err := bcrypt.GenerateFromPassword([]byte(hex.EncodeToString(filler)), bcrypt.DefaultCost); err == nil {
			store.decoy = hash
		}
	}

	return store
}

// Authenticate reports whether the secret matches the client.
func (s *Store) Authenticate(id, secret string) bool {
	hash, known := s.hashes[id]
	if !known {
		// Deliberately still hash, to keep the timing flat.
		if s.decoy != nil {
			_ = bcrypt.CompareHashAndPassword(s.decoy, []byte(secret))
		}
		return false
	}
	return bcrypt.CompareHashAndPassword(hash, []byte(secret)) == nil
}

// Len reports how many clients are configured, for startup logging.
func (s *Store) Len() int { return len(s.hashes) }

// newTokenID returns a random jti so individual tokens are traceable in logs
// and revocable if a deny list is ever added.
func newTokenID() string {
	buf := make([]byte, 16)
	if _, err := rand.Read(buf); err != nil {
		return ""
	}
	return hex.EncodeToString(buf)
}
