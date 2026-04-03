package identitytest

import (
	"context"

	"github.com/stretchr/testify/mock"
	"github.com/BEDOLAGA-DEV/RemnaCore/internal/domain/identity"
	"github.com/BEDOLAGA-DEV/RemnaCore/pkg/domainevent"
)

// MockRepository is a testify mock implementation of identity.Repository.
// It is exported so both in-package and external test files can share it.
type MockRepository struct {
	mock.Mock
}

func (m *MockRepository) CreateUser(ctx context.Context, user *identity.PlatformUser) error {
	args := m.Called(ctx, user)
	return args.Error(0)
}

func (m *MockRepository) GetUserByID(ctx context.Context, id string) (*identity.PlatformUser, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*identity.PlatformUser), args.Error(1)
}

func (m *MockRepository) GetUserByEmail(ctx context.Context, email string) (*identity.PlatformUser, error) {
	args := m.Called(ctx, email)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*identity.PlatformUser), args.Error(1)
}

func (m *MockRepository) GetUserByTelegramID(ctx context.Context, telegramID int64) (*identity.PlatformUser, error) {
	args := m.Called(ctx, telegramID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*identity.PlatformUser), args.Error(1)
}

func (m *MockRepository) UpdateUser(ctx context.Context, user *identity.PlatformUser) error {
	args := m.Called(ctx, user)
	return args.Error(0)
}

func (m *MockRepository) ListUsers(ctx context.Context, limit, offset int) ([]*identity.PlatformUser, error) {
	args := m.Called(ctx, limit, offset)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*identity.PlatformUser), args.Error(1)
}

func (m *MockRepository) CreateSession(ctx context.Context, session *identity.Session) error {
	args := m.Called(ctx, session)
	return args.Error(0)
}

func (m *MockRepository) GetSessionByRefreshToken(ctx context.Context, token string) (*identity.Session, error) {
	args := m.Called(ctx, token)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*identity.Session), args.Error(1)
}

func (m *MockRepository) DeleteSession(ctx context.Context, id string) error {
	args := m.Called(ctx, id)
	return args.Error(0)
}

func (m *MockRepository) DeleteUserSessions(ctx context.Context, userID string) error {
	args := m.Called(ctx, userID)
	return args.Error(0)
}

func (m *MockRepository) CreateEmailVerification(ctx context.Context, v *identity.EmailVerification) error {
	args := m.Called(ctx, v)
	return args.Error(0)
}

func (m *MockRepository) GetEmailVerification(ctx context.Context, token string) (*identity.EmailVerification, error) {
	args := m.Called(ctx, token)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*identity.EmailVerification), args.Error(1)
}

func (m *MockRepository) DeleteEmailVerification(ctx context.Context, id string) error {
	args := m.Called(ctx, id)
	return args.Error(0)
}

func (m *MockRepository) CreatePasswordReset(ctx context.Context, pr *identity.PasswordReset) error {
	args := m.Called(ctx, pr)
	return args.Error(0)
}

func (m *MockRepository) GetPasswordResetByToken(ctx context.Context, token string) (*identity.PasswordReset, error) {
	args := m.Called(ctx, token)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*identity.PasswordReset), args.Error(1)
}

func (m *MockRepository) DeletePasswordReset(ctx context.Context, id string) error {
	args := m.Called(ctx, id)
	return args.Error(0)
}

func (m *MockRepository) DeleteUserPasswordResets(ctx context.Context, userID string) error {
	args := m.Called(ctx, userID)
	return args.Error(0)
}

// MockPublisher is a testify mock implementation of domainevent.Publisher.
type MockPublisher struct {
	mock.Mock
}

func (m *MockPublisher) Publish(ctx context.Context, event domainevent.Event) error {
	args := m.Called(ctx, event)
	return args.Error(0)
}

// Ensure MockPublisher satisfies domainevent.Publisher at compile time.
var _ domainevent.Publisher = (*MockPublisher)(nil)
