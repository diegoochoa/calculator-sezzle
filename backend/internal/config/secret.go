package config

import "golang.org/x/crypto/bcrypt"

// HashCost is deliberately the library default. It is the single knob trading
// login latency against offline-cracking resistance.
const HashCost = bcrypt.DefaultCost

// hashSecret bcrypt-hashes a plaintext client secret.
func hashSecret(secret string) ([]byte, error) {
	return bcrypt.GenerateFromPassword([]byte(secret), HashCost)
}

// HashSecret exposes hashing so operators can mint values for CALC_CLIENTS.
func HashSecret(secret string) ([]byte, error) { return hashSecret(secret) }
