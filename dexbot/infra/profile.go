/******************************************************************************
 * File Name       : profile.go
 * File Path       : infra/profile.go
 *
 * Author          : deepseek-4.0-pro
 * Owner           : Chalearm Saelim
 * Version         : 1.0.0
 * Created Date    : 2026-07-05 10:00:00 (UTC+7)
 *
 * Description     :
 *   User profile system. Profile ID = SHA256(first 16 characters of
 *   the private key). Profiles are stored in PostgreSQL user_profiles
 *   table. LookupOrCreate returns existing or creates new.
 *
 *   The private key NEVER leaves config.env. Only the 16-char prefix
 *   is hashed to create the profile identifier.
 *
 * Usage : go test ./infra -v -run Profile
 ******************************************************************************/
package infra

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"time"
)

// Profile represents a user profile stored in the database.
type Profile struct {
	ID         string    `json:"id"`          // SHA256(first16chars of private key)
	MaskedKey  string    `json:"masked_key"`  // first 8 chars + "*****"
	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at"`
	TotalUSD   float64   `json:"total_usd"`   // last known total balance
}

// ProfileID computes SHA256(first 16 chars of private key).
func ProfileID(privateKey string) string {
	if len(privateKey) < 16 {
		return ""
	}
	prefix := privateKey[:16]
	hash := sha256.Sum256([]byte(prefix))
	return hex.EncodeToString(hash[:])
}

// CreateUserProfilesTable ensures the user_profiles table exists.
func CreateUserProfilesTable() {
	if DB == nil {
		return
	}
	q := `CREATE TABLE IF NOT EXISTS user_profiles (
		id VARCHAR(64) PRIMARY KEY,
		masked_key VARCHAR(32) NOT NULL DEFAULT '',
		created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
		updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
		total_usd DOUBLE PRECISION NOT NULL DEFAULT 0
	)`
	if _, err := DB.Exec(q); err != nil {
		Warn("CreateUserProfilesTable: " + err.Error())
	} else {
		Info("user_profiles table ready")
	}
}

// LookupOrCreateProfile finds an existing profile by ID or creates a new one.
// Returns the profile and whether it was newly created.
func LookupOrCreateProfile(pk string) (*Profile, bool) {
	id := ProfileID(pk)
	if id == "" {
		return &Profile{ID: "anon", MaskedKey: "no-key"}, false
	}

	if DB == nil {
		// No DB — return in-memory profile
		return &Profile{
			ID: id,
			MaskedKey: pk[:8] + "*****",
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		}, true
	}

	CreateUserProfilesTable()

	masked := pk[:8] + "*****"
	if len(pk) < 8 {
		masked = pk + "*****"
	}

	// Try to find existing
	var existing Profile
	err := DB.QueryRow(`SELECT id, masked_key, created_at, updated_at, total_usd FROM user_profiles WHERE id=$1`, id).
		Scan(&existing.ID, &existing.MaskedKey, &existing.CreatedAt, &existing.UpdatedAt, &existing.TotalUSD)
	if err == nil {
		// Found — update timestamp
		DB.Exec(`UPDATE user_profiles SET updated_at=NOW() WHERE id=$1`, id)
		existing.UpdatedAt = time.Now()
		Info(fmt.Sprintf("Profile found: %s (created %s)", existing.MaskedKey, existing.CreatedAt.Format(time.RFC3339)))
		return &existing, false
	}

	// Not found — create new
	now := time.Now()
	_, err = DB.Exec(`INSERT INTO user_profiles (id, masked_key, created_at, updated_at, total_usd) VALUES ($1,$2,$3,$4,$5)`,
		id, masked, now, now, 0.0)
	if err != nil {
		Error("Failed to create profile: " + err.Error())
		return &Profile{ID: id, MaskedKey: masked, CreatedAt: now, UpdatedAt: now}, true
	}

	Info(fmt.Sprintf("Profile created: %s (id=%s)", masked, id[:16]+"..."))
	return &Profile{
		ID:        id,
		MaskedKey: masked,
		CreatedAt: now,
		UpdatedAt: now,
	}, true
}

// UpdateProfileBalance updates the total_usd for a profile.
func UpdateProfileBalance(id string, totalUSD float64) {
	if DB == nil {
		return
	}
	DB.Exec(`UPDATE user_profiles SET total_usd=$1, updated_at=NOW() WHERE id=$2`, totalUSD, id)
}

// DeleteProfile removes a profile from the database.
func DeleteProfile(id string) error {
	if DB == nil {
		return fmt.Errorf("no database connection")
	}
	_, err := DB.Exec(`DELETE FROM user_profiles WHERE id=$1`, id)
	return err
}
