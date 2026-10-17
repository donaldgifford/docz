package store

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
)

// ErrNotFound is returned by the OAuth reads for a row that does not exist;
// callers test it with errors.Is.
var ErrNotFound = errors.New("not found")

// RefreshToken is a row of oauth_refresh_tokens as the authorization server
// reads it (DESIGN-0023 §5): the hash, never the token.
type RefreshToken struct {
	ID         int64
	TokenHash  string
	FamilyID   string
	UserID     int64
	ClientID   string
	Scope      string
	IssuedAt   time.Time
	LastUsedAt *time.Time
	RevokedAt  *time.Time
}

// TokenUser is the durable identity row a token names.
type TokenUser struct {
	ID       int64
	Provider string
	Subject  string
	Email    string
	Login    string
}

// InsertRefreshToken records a new family's first token.
func (s *Store) InsertRefreshToken(ctx context.Context, hash, familyID string, userID int64, clientID, scope string) error {
	_, err := s.q.InsertRefreshToken(ctx, InsertRefreshTokenParams{
		TokenHash: hash, FamilyID: familyID, UserID: userID, ClientID: clientID, Scope: scope,
	})
	if err != nil {
		return fmt.Errorf("insert refresh token: %w", err)
	}
	return nil
}

// GetRefreshToken reads a token by its hash; ErrNotFound when there is none.
func (s *Store) GetRefreshToken(ctx context.Context, hash string) (RefreshToken, error) {
	row, err := s.q.GetRefreshTokenByHash(ctx, hash)
	if errors.Is(err, pgx.ErrNoRows) {
		return RefreshToken{}, ErrNotFound
	}
	if err != nil {
		return RefreshToken{}, fmt.Errorf("get refresh token: %w", err)
	}
	return toRefreshToken(&row), nil
}

// RotateRefreshToken marks the live token oldHash used and inserts newHash in
// its family with scope, in one statement. rotated is false when oldHash was
// not live, which the caller treats as reuse.
func (s *Store) RotateRefreshToken(ctx context.Context, oldHash, newHash, scope string) (token RefreshToken, rotated bool, err error) {
	row, err := s.q.RotateRefreshToken(ctx, RotateRefreshTokenParams{OldHash: oldHash, NewHash: newHash, Scope: scope})
	if errors.Is(err, pgx.ErrNoRows) {
		return RefreshToken{}, false, nil
	}
	if err != nil {
		return RefreshToken{}, false, fmt.Errorf("rotate refresh token: %w", err)
	}
	return toRefreshToken(&row), true, nil
}

// RevokeRefreshFamily revokes every live token in the family.
func (s *Store) RevokeRefreshFamily(ctx context.Context, familyID string) error {
	if _, err := s.q.RevokeRefreshFamily(ctx, familyID); err != nil {
		return fmt.Errorf("revoke refresh family: %w", err)
	}
	return nil
}

// UpsertConsent remembers that the user approved the client for scope.
func (s *Store) UpsertConsent(ctx context.Context, userID int64, clientID, scope string) error {
	if err := s.q.UpsertConsent(ctx, UpsertConsentParams{UserID: userID, ClientID: clientID, Scope: scope}); err != nil {
		return fmt.Errorf("upsert consent: %w", err)
	}
	return nil
}

// GetUser reads a user by id; ErrNotFound when there is none.
func (s *Store) GetUser(ctx context.Context, id int64) (TokenUser, error) {
	row, err := s.q.GetUserByID(ctx, id)
	if errors.Is(err, pgx.ErrNoRows) {
		return TokenUser{}, ErrNotFound
	}
	if err != nil {
		return TokenUser{}, fmt.Errorf("get user %d: %w", id, err)
	}
	return TokenUser{ID: row.ID, Provider: row.Provider, Subject: row.Subject, Email: row.Email.String, Login: row.Login.String}, nil
}

func toRefreshToken(row *OauthRefreshToken) RefreshToken {
	t := RefreshToken{
		ID: row.ID, TokenHash: row.TokenHash, FamilyID: row.FamilyID, UserID: row.UserID,
		ClientID: row.ClientID, Scope: row.Scope, IssuedAt: row.IssuedAt.Time,
	}
	if row.LastUsedAt.Valid {
		used := row.LastUsedAt.Time
		t.LastUsedAt = &used
	}
	if row.RevokedAt.Valid {
		revoked := row.RevokedAt.Time
		t.RevokedAt = &revoked
	}
	return t
}
