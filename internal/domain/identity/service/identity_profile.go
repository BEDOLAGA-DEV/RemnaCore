package service

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/BEDOLAGA-DEV/RemnaCore/internal/domain/identity/aggregate"
	"github.com/BEDOLAGA-DEV/RemnaCore/pkg/authutil"
	"github.com/BEDOLAGA-DEV/RemnaCore/pkg/domainevent"
)

// VerifyEmail validates the token, marks the user's email as verified, removes
// the verification record, and publishes an EmailVerified event. The user read
// (with FOR UPDATE lock), mutation, update, and event publish are all inside a
// single transaction to prevent TOCTOU races.
func (s *Service) VerifyEmail(ctx context.Context, token string) error {
	// Verification token lookup is read-only and not subject to concurrent mutation.
	verification, err := s.repo.GetEmailVerification(ctx, token)
	if err != nil {
		return fmt.Errorf("finding verification: %w", err)
	}

	if verification.IsExpiredAt(s.clock.Now()) {
		return ErrTokenExpired
	}

	return s.txRunner.RunInTx(ctx, func(txCtx context.Context) error {
		user, err := s.repo.GetUserByIDForUpdate(txCtx, verification.UserID)
		if err != nil {
			return fmt.Errorf("finding user: %w", err)
		}

		now := s.clock.Now()
		user.VerifyEmail(now)

		if err := s.repo.UpdateUser(txCtx, user); err != nil {
			return fmt.Errorf("updating user: %w", err)
		}
		if err := s.repo.DeleteEmailVerification(txCtx, verification.ID); err != nil {
			return fmt.Errorf("deleting verification: %w", err)
		}
		return domainevent.PublishAll(txCtx, s.publisher, user)
	})
}

// GetMe retrieves the authenticated user's profile by ID.
func (s *Service) GetMe(ctx context.Context, userID string) (*aggregate.PlatformUser, error) {
	user, err := s.repo.GetUserByID(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("finding user: %w", err)
	}
	return user, nil
}

// GetByTelegramID retrieves a user by their linked Telegram ID.
func (s *Service) GetByTelegramID(ctx context.Context, telegramID int64) (*aggregate.PlatformUser, error) {
	user, err := s.repo.GetUserByTelegramID(ctx, telegramID)
	if err != nil {
		return nil, fmt.Errorf("finding user by telegram id: %w", err)
	}
	return user, nil
}

// UpdateDisplayName updates the user's display name. The read (with FOR UPDATE
// lock), mutation, and update are all inside a single transaction to prevent
// TOCTOU races.
func (s *Service) UpdateDisplayName(ctx context.Context, userID, displayName string) error {
	return s.txRunner.RunInTx(ctx, func(txCtx context.Context) error {
		user, err := s.repo.GetUserByIDForUpdate(txCtx, userID)
		if err != nil {
			return fmt.Errorf("finding user: %w", err)
		}
		if err := user.ChangeDisplayName(displayName, s.clock.Now()); err != nil {
			return err
		}
		return s.repo.UpdateUser(txCtx, user)
	})
}

// LinkTelegram links a Telegram ID to the user's account. The read (with FOR
// UPDATE lock), mutation, update, and event publish are all inside a single
// transaction to prevent TOCTOU races.
func (s *Service) LinkTelegram(ctx context.Context, userID string, telegramID int64) error {
	return s.txRunner.RunInTx(ctx, func(txCtx context.Context) error {
		user, err := s.repo.GetUserByIDForUpdate(txCtx, userID)
		if err != nil {
			return fmt.Errorf("finding user: %w", err)
		}
		if err := user.LinkTelegram(telegramID, s.clock.Now()); err != nil {
			return err
		}
		if err := s.repo.UpdateUser(txCtx, user); err != nil {
			return fmt.Errorf("updating user: %w", err)
		}
		return domainevent.PublishAll(txCtx, s.publisher, user)
	})
}

// UnlinkTelegram removes the Telegram ID from the user's account. The read
// (with FOR UPDATE lock), mutation, update, and event publish are all inside a
// single transaction to prevent TOCTOU races.
func (s *Service) UnlinkTelegram(ctx context.Context, userID string) error {
	return s.txRunner.RunInTx(ctx, func(txCtx context.Context) error {
		user, err := s.repo.GetUserByIDForUpdate(txCtx, userID)
		if err != nil {
			return fmt.Errorf("finding user: %w", err)
		}
		if err := user.UnlinkTelegram(s.clock.Now()); err != nil {
			return err
		}
		if err := s.repo.UpdateUser(txCtx, user); err != nil {
			return fmt.Errorf("updating user: %w", err)
		}
		return domainevent.PublishAll(txCtx, s.publisher, user)
	})
}

// ListUsers returns a paginated list of all users. Intended for admin endpoints.
func (s *Service) ListUsers(ctx context.Context, limit, offset int) ([]*aggregate.PlatformUser, error) {
	users, err := s.repo.ListUsers(ctx, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("listing users: %w", err)
	}
	return users, nil
}

// RequestPasswordReset generates a password reset token for the given email and
// publishes a PasswordResetRequested event. If the email is not found, no error
// is returned to prevent user enumeration.
func (s *Service) RequestPasswordReset(ctx context.Context, email string) error {
	user, err := s.repo.GetUserByEmail(ctx, email)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			// Silently succeed to prevent email enumeration.
			return nil
		}
		return fmt.Errorf("finding user by email: %w", err)
	}

	now := s.clock.Now()
	reset, err := aggregate.NewPasswordReset(user.ID, user.Email, now)
	if err != nil {
		return fmt.Errorf("creating password reset: %w", err)
	}

	return s.txRunner.RunInTx(ctx, func(txCtx context.Context) error {
		// Remove any existing reset tokens for this user.
		if err := s.repo.DeleteUserPasswordResets(txCtx, user.ID); err != nil {
			return fmt.Errorf("clearing existing resets: %w", err)
		}
		if err := s.repo.CreatePasswordReset(txCtx, reset); err != nil {
			return fmt.Errorf("persisting password reset: %w", err)
		}
		// Notification plugins listen for this event to send the actual email.
		if err := s.publisher.Publish(txCtx, NewPasswordResetRequestedEvent(user.ID, user.Email, reset.Token, now)); err != nil {
			return fmt.Errorf("publishing %s: %w", aggregate.EventPasswordResetRequested, err)
		}
		return nil
	})
}

// ResetPassword validates the reset token, sets the new password, invalidates
// all existing sessions, and publishes a PasswordReset event. The user read
// (with FOR UPDATE lock), mutation, update, session invalidation, and event
// publish are all inside a single transaction to prevent TOCTOU races.
func (s *Service) ResetPassword(ctx context.Context, token, newPassword string) error {
	// Token lookup and validation are read-only and not subject to concurrent mutation.
	reset, err := s.repo.GetPasswordResetByToken(ctx, token)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return ErrPasswordResetNotFound
		}
		return fmt.Errorf("finding password reset: %w", err)
	}

	if reset.IsExpiredAt(s.clock.Now()) {
		return ErrPasswordResetExpired
	}

	if err := aggregate.ValidatePassword(newPassword); err != nil {
		return err
	}

	hash, err := authutil.HashPassword(newPassword)
	if err != nil {
		return fmt.Errorf("hashing password: %w", err)
	}

	return s.txRunner.RunInTx(ctx, func(txCtx context.Context) error {
		user, err := s.repo.GetUserByIDForUpdate(txCtx, reset.UserID)
		if err != nil {
			return fmt.Errorf("finding user: %w", err)
		}

		now := s.clock.Now()
		user.ChangePassword(hash, now)
		// Spec: a successful password reset clears the forced-change flag so the
		// user is not prompted again after completing the reset flow.
		user.MustChangePassword = false

		if err := s.repo.UpdateUser(txCtx, user); err != nil {
			return fmt.Errorf("updating user: %w", err)
		}
		if err := s.repo.DeleteUserSessions(txCtx, user.ID); err != nil {
			return fmt.Errorf("invalidating sessions: %w", err)
		}
		if err := s.repo.DeletePasswordReset(txCtx, reset.ID); err != nil {
			return fmt.Errorf("deleting password reset: %w", err)
		}
		if err := s.publisher.Publish(txCtx, NewPasswordResetEvent(user.ID, now)); err != nil {
			return fmt.Errorf("publishing %s: %w", aggregate.EventPasswordReset, err)
		}
		return domainevent.PublishAll(txCtx, s.publisher, user)
	})
}

// NewPasswordResetRequestedEvent creates an event when a user requests a
// password reset. Notification plugins listen for this to send the reset email.
func NewPasswordResetRequestedEvent(userID, email, token string, now time.Time) domainevent.Event {
	return domainevent.NewTyped(aggregate.PasswordResetRequestedPayload{
		UserID: userID,
		Email:  email,
		Token:  token,
	}, now, userID)
}

// NewPasswordResetEvent creates an event when a password has been successfully
// reset.
func NewPasswordResetEvent(userID string, now time.Time) domainevent.Event {
	return domainevent.NewTyped(aggregate.PasswordResetPayload{
		UserID: userID,
	}, now, userID)
}
