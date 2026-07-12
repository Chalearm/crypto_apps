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
	ID        string    `json:"id"`         // SHA256(first16chars of private key)
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
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
		created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
		updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
	)`
	if _, err := DB.Exec(q); err != nil {
		Warn("CreateUserProfilesTable: " + err.Error())
	} else {
		Info("user_profiles table ready")
	}
}

// LookupOrCreateProfile finds an existing profile by ID or creates a new one.
func LookupOrCreateProfile(pk string) (*Profile, bool) {
	id := ProfileID(pk)
	if id == "" {
		return &Profile{}, false
	}

	if DB == nil {
		now := time.Now()
		return &Profile{ID: id, CreatedAt: now, UpdatedAt: now}, true
	}

	CreateUserProfilesTable()

	// Try to find existing
	var existing Profile
	err := DB.QueryRow(`SELECT id, created_at, updated_at FROM user_profiles WHERE id=$1`, id).
		Scan(&existing.ID, &existing.CreatedAt, &existing.UpdatedAt)
	if err == nil {
		DB.Exec(`UPDATE user_profiles SET updated_at=NOW() WHERE id=$1`, id)
		existing.UpdatedAt = time.Now()
		Info(fmt.Sprintf("Profile found: %s (created %s)", id[:8], existing.CreatedAt.Format(time.RFC3339)))
		return &existing, false
	}

	// Not found — create new
	now := time.Now()
	_, err = DB.Exec(`INSERT INTO user_profiles (id, created_at, updated_at) VALUES ($1,$2,$3)`,
		id, now, now)
	if err != nil {
		Error("Failed to create profile: " + err.Error())
		return &Profile{ID: id, CreatedAt: now, UpdatedAt: now}, true
	}

	Info(fmt.Sprintf("Profile created: id=%s", id[:16]+"..."))
	return &Profile{ID: id, CreatedAt: now, UpdatedAt: now}, true
}
func UpdateProfileBalance(id string, totalUSD float64) {}

// DeleteProfile removes a profile from the database.
func DeleteProfile(id string) error {
	if DB == nil {
		return fmt.Errorf("no database connection")
	}
	_, err := DB.Exec(`DELETE FROM user_profiles WHERE id=$1`, id)
	return err
}
