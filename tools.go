//go:build tools

package tools

// This file pins indirect dependencies so `go mod tidy` does not remove them
// before they are imported in application code. Remove entries as real imports
// are added throughout the codebase.

import (
	_ "github.com/ThreeDotsLabs/watermill"
	_ "github.com/ThreeDotsLabs/watermill-nats/v2/pkg/nats"
	_ "github.com/go-chi/chi/v5"
	_ "github.com/golang-jwt/jwt/v5"
	_ "github.com/google/uuid"
	_ "github.com/hashicorp/go-retryablehttp"
	_ "github.com/jackc/pgx/v5"
	_ "github.com/knadh/koanf/parsers/yaml"
	_ "github.com/knadh/koanf/providers/env"
	_ "github.com/knadh/koanf/v2"
	_ "github.com/nats-io/nats.go"
	_ "github.com/prometheus/client_golang/prometheus"
	_ "github.com/redis/go-redis/v9"
	_ "github.com/rs/zerolog"
	_ "github.com/sony/gobreaker/v2"
	_ "github.com/stretchr/testify/assert"
	_ "go.uber.org/fx"
)
