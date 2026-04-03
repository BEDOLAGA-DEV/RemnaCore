package remnawave

import (
	"context"
	"fmt"
	"time"

	"github.com/sony/gobreaker/v2"
)

const (
	// CBName is the circuit breaker instance name.
	CBName = "remnawave"

	// CBMaxRequests is the number of requests allowed in the half-open state.
	CBMaxRequests = 3

	// CBInterval is the cyclic period of the closed state for clearing internal
	// counts. If zero, the internal counts are never cleared.
	CBInterval = 30 * time.Second

	// CBTimeout is the duration the circuit stays open before transitioning to
	// half-open.
	CBTimeout = 10 * time.Second

	// CBConsecutiveFailures is the number of consecutive failures required to
	// trip the breaker from closed to open.
	CBConsecutiveFailures = 5
)

// ResilientClient wraps a Client with a circuit breaker that opens after
// consecutive failures and prevents cascading failures.
type ResilientClient struct {
	client *Client
	cb     *gobreaker.CircuitBreaker[any]
}

// NewResilientClient wraps the provided Client with a circuit breaker.
func NewResilientClient(client *Client) *ResilientClient {
	settings := gobreaker.Settings{
		Name:        CBName,
		MaxRequests: CBMaxRequests,
		Interval:    CBInterval,
		Timeout:     CBTimeout,
		ReadyToTrip: func(counts gobreaker.Counts) bool {
			return counts.ConsecutiveFailures >= CBConsecutiveFailures
		},
	}

	return &ResilientClient{
		client: client,
		cb:     gobreaker.NewCircuitBreaker[any](settings),
	}
}

// CreateUser provisions a new VPN user through the circuit breaker.
func (rc *ResilientClient) CreateUser(ctx context.Context, req CreateUserRequest) (*RemnawaveUser, error) {
	result, err := rc.cb.Execute(func() (any, error) {
		return rc.client.CreateUser(ctx, req)
	})
	if err != nil {
		return nil, fmt.Errorf("circuit breaker: %w", err)
	}
	user, ok := result.(*RemnawaveUser)
	if !ok {
		return nil, fmt.Errorf("circuit breaker: unexpected result type %T", result)
	}
	return user, nil
}

// GetNodes returns all proxy nodes through the circuit breaker.
func (rc *ResilientClient) GetNodes(ctx context.Context) ([]RemnawaveNode, error) {
	result, err := rc.cb.Execute(func() (any, error) {
		return rc.client.GetNodes(ctx)
	})
	if err != nil {
		return nil, fmt.Errorf("circuit breaker: %w", err)
	}
	nodes, ok := result.([]RemnawaveNode)
	if !ok {
		return nil, fmt.Errorf("circuit breaker: unexpected result type %T", result)
	}
	return nodes, nil
}

// GetUserByUUID retrieves a single VPN user with traffic stats through the
// circuit breaker.
func (rc *ResilientClient) GetUserByUUID(ctx context.Context, uuid string) (*RemnawaveUserWithTraffic, error) {
	result, err := rc.cb.Execute(func() (any, error) {
		return rc.client.GetUserByUUID(ctx, uuid)
	})
	if err != nil {
		return nil, fmt.Errorf("circuit breaker: %w", err)
	}
	user, ok := result.(*RemnawaveUserWithTraffic)
	if !ok {
		return nil, fmt.Errorf("circuit breaker: unexpected result type %T", result)
	}
	return user, nil
}

// UpdateUser modifies an existing VPN user through the circuit breaker.
func (rc *ResilientClient) UpdateUser(ctx context.Context, req UpdateUserRequest) (*RemnawaveUser, error) {
	result, err := rc.cb.Execute(func() (any, error) {
		return rc.client.UpdateUser(ctx, req)
	})
	if err != nil {
		return nil, fmt.Errorf("circuit breaker: %w", err)
	}
	user, ok := result.(*RemnawaveUser)
	if !ok {
		return nil, fmt.Errorf("circuit breaker: unexpected result type %T", result)
	}
	return user, nil
}

// DeleteUser removes a VPN user through the circuit breaker.
func (rc *ResilientClient) DeleteUser(ctx context.Context, uuid string) error {
	_, err := rc.cb.Execute(func() (any, error) {
		return nil, rc.client.DeleteUser(ctx, uuid)
	})
	if err != nil {
		return fmt.Errorf("circuit breaker: %w", err)
	}
	return nil
}

// EnableUser activates a VPN user through the circuit breaker.
func (rc *ResilientClient) EnableUser(ctx context.Context, uuid string) error {
	_, err := rc.cb.Execute(func() (any, error) {
		return nil, rc.client.EnableUser(ctx, uuid)
	})
	if err != nil {
		return fmt.Errorf("circuit breaker: %w", err)
	}
	return nil
}

// DisableUser deactivates a VPN user through the circuit breaker.
func (rc *ResilientClient) DisableUser(ctx context.Context, uuid string) error {
	_, err := rc.cb.Execute(func() (any, error) {
		return nil, rc.client.DisableUser(ctx, uuid)
	})
	if err != nil {
		return fmt.Errorf("circuit breaker: %w", err)
	}
	return nil
}
