// Package paymenttest provides mock implementations of payment domain interfaces
// for use in unit tests. All mocks use testify/mock.
package paymenttest

import (
	"context"

	"github.com/stretchr/testify/mock"
	"github.com/BEDOLAGA-DEV/RemnaCore/internal/domain/payment"
	"github.com/BEDOLAGA-DEV/RemnaCore/pkg/domainevent"
)

// --- MockPaymentRepo ---

// MockPaymentRepo is a testify/mock implementation of payment.PaymentRepository.
type MockPaymentRepo struct {
	mock.Mock
}

func (m *MockPaymentRepo) CreatePayment(ctx context.Context, record *payment.PaymentRecord) error {
	args := m.Called(ctx, record)
	return args.Error(0)
}

func (m *MockPaymentRepo) GetPaymentByID(ctx context.Context, id string) (*payment.PaymentRecord, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*payment.PaymentRecord), args.Error(1)
}

func (m *MockPaymentRepo) GetPaymentByExternalID(ctx context.Context, provider, externalID string) (*payment.PaymentRecord, error) {
	args := m.Called(ctx, provider, externalID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*payment.PaymentRecord), args.Error(1)
}

func (m *MockPaymentRepo) UpdatePayment(ctx context.Context, record *payment.PaymentRecord) error {
	args := m.Called(ctx, record)
	return args.Error(0)
}

func (m *MockPaymentRepo) CreateWebhookLog(ctx context.Context, log *payment.WebhookLog) error {
	args := m.Called(ctx, log)
	return args.Error(0)
}

func (m *MockPaymentRepo) GetWebhookLog(ctx context.Context, provider, externalID string) (*payment.WebhookLog, error) {
	args := m.Called(ctx, provider, externalID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*payment.WebhookLog), args.Error(1)
}

func (m *MockPaymentRepo) UpdateWebhookLog(ctx context.Context, log *payment.WebhookLog) error {
	args := m.Called(ctx, log)
	return args.Error(0)
}

// --- MockEventPublisher ---

// MockEventPublisher is a testify/mock implementation of domainevent.Publisher.
type MockEventPublisher struct {
	mock.Mock
}

func (m *MockEventPublisher) Publish(ctx context.Context, event domainevent.Event) error {
	args := m.Called(ctx, event)
	return args.Error(0)
}
