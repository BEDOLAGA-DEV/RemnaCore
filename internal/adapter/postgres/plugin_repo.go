package postgres

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/BEDOLAGA-DEV/RemnaCore/internal/adapter/postgres/gen"
	"github.com/BEDOLAGA-DEV/RemnaCore/internal/plugin"
	"github.com/BEDOLAGA-DEV/RemnaCore/pkg/pgutil"
)

// PluginRepository implements plugin.PluginRepository backed by PostgreSQL.
type PluginRepository struct {
	pool    *pgxpool.Pool
	queries *gen.Queries
}

// NewPluginRepository returns a new PluginRepository using the given pool.
func NewPluginRepository(pool *pgxpool.Pool) *PluginRepository {
	return &PluginRepository{
		pool:    pool,
		queries: gen.New(pool),
	}
}

// permissionsToStrings converts typed PermissionScope values to plain strings
// for PostgreSQL TEXT[] storage.
func permissionsToStrings(scopes []plugin.PermissionScope) []string {
	out := make([]string, len(scopes))
	for i, s := range scopes {
		out[i] = string(s)
	}
	return out
}

// stringsToPermissions converts plain strings back to typed PermissionScope
// values.
func stringsToPermissions(ss []string) []plugin.PermissionScope {
	out := make([]plugin.PermissionScope, len(ss))
	for i, s := range ss {
		out[i] = plugin.PermissionScope(s)
	}
	return out
}

// pluginRowToDomain converts a sqlc-generated row to the domain Plugin model.
func pluginRowToDomain(row gen.PluginsPluginRegistry) (*plugin.Plugin, error) {
	var manifest plugin.Manifest
	if err := json.Unmarshal(row.Manifest, &manifest); err != nil {
		return nil, err
	}

	cfg := make(map[string]string)
	if len(row.Config) > 0 {
		if err := json.Unmarshal(row.Config, &cfg); err != nil {
			return nil, err
		}
	}

	return &plugin.Plugin{
		ID:          pgutil.PgtypeToUUID(row.ID),
		Slug:        row.Slug,
		Name:        row.Name,
		Version:     row.Version,
		Description: pgutil.DerefStr(row.Description),
		Author:      pgutil.DerefStr(row.Author),
		License:     pgutil.DerefStr(row.License),
		SDKVersion:  pgutil.DerefStr(row.SdkVersion),
		Lang:        pgutil.DerefStr(row.Lang),
		WASMBytes:   row.WasmBytes,
		WASMHash:    pgutil.DerefStr(row.WasmHash),
		Manifest:    &manifest,
		Status:      plugin.PluginStatus(row.Status),
		Config:      cfg,
		Permissions: stringsToPermissions(row.Permissions),
		ErrorLog:    pgutil.DerefStr(row.ErrorLog),
		InstalledAt: pgutil.PgtypeToTime(row.InstalledAt),
		EnabledAt:   pgutil.PgtypeToOptTime(row.EnabledAt),
		UpdatedAt:   pgutil.PgtypeToTime(row.UpdatedAt),
	}, nil
}

func (r *PluginRepository) Create(ctx context.Context, p *plugin.Plugin) error {
	manifestJSON, err := json.Marshal(p.Manifest)
	if err != nil {
		return fmt.Errorf("marshal plugin manifest: %w", err)
	}

	configJSON, err := json.Marshal(p.Config)
	if err != nil {
		return fmt.Errorf("marshal plugin config: %w", err)
	}

	err = r.queries.CreatePlugin(ctx, gen.CreatePluginParams{
		ID:          pgutil.UUIDToPgtype(p.ID),
		Slug:        p.Slug,
		Name:        p.Name,
		Version:     p.Version,
		Description: pgutil.StrPtrOrNil(p.Description),
		Author:      pgutil.StrPtrOrNil(p.Author),
		License:     pgutil.StrPtrOrNil(p.License),
		SdkVersion:  pgutil.StrPtrOrNil(p.SDKVersion),
		Lang:        pgutil.StrPtrOrNil(p.Lang),
		WasmBytes:   p.WASMBytes,
		WasmHash:    pgutil.StrPtrOrNil(p.WASMHash),
		Manifest:    manifestJSON,
		Status:      string(p.Status),
		Config:      configJSON,
		Permissions: permissionsToStrings(p.Permissions),
		InstalledAt: pgutil.TimeToPgtype(p.InstalledAt),
		EnabledAt:   pgutil.OptTimeToPgtype(p.EnabledAt),
		UpdatedAt:   pgutil.TimeToPgtype(p.UpdatedAt),
	})
	return pgutil.MapErr(err, "create plugin", plugin.ErrPluginNotFound)
}

func (r *PluginRepository) GetByID(ctx context.Context, id string) (*plugin.Plugin, error) {
	row, err := r.queries.GetPluginByID(ctx, pgutil.UUIDToPgtype(id))
	if err != nil {
		return nil, pgutil.MapErr(err, "get plugin by id", plugin.ErrPluginNotFound)
	}
	return pluginRowToDomain(row)
}

func (r *PluginRepository) GetBySlug(ctx context.Context, slug string) (*plugin.Plugin, error) {
	row, err := r.queries.GetPluginBySlug(ctx, slug)
	if err != nil {
		return nil, pgutil.MapErr(err, "get plugin by slug", plugin.ErrPluginNotFound)
	}
	return pluginRowToDomain(row)
}

func (r *PluginRepository) GetAll(ctx context.Context) ([]*plugin.Plugin, error) {
	rows, err := r.queries.GetAllPlugins(ctx)
	if err != nil {
		return nil, pgutil.MapErr(err, "get all plugins", plugin.ErrPluginNotFound)
	}
	plugins := make([]*plugin.Plugin, len(rows))
	for i, row := range rows {
		p, convErr := pluginRowToDomain(row)
		if convErr != nil {
			return nil, convErr
		}
		plugins[i] = p
	}
	return plugins, nil
}

func (r *PluginRepository) GetEnabled(ctx context.Context) ([]*plugin.Plugin, error) {
	rows, err := r.queries.GetEnabledPlugins(ctx)
	if err != nil {
		return nil, pgutil.MapErr(err, "get enabled plugins", plugin.ErrPluginNotFound)
	}
	plugins := make([]*plugin.Plugin, len(rows))
	for i, row := range rows {
		p, convErr := pluginRowToDomain(row)
		if convErr != nil {
			return nil, convErr
		}
		plugins[i] = p
	}
	return plugins, nil
}

func (r *PluginRepository) UpdateStatus(ctx context.Context, id string, status plugin.PluginStatus, errorLog string, enabledAt *time.Time) error {
	err := r.queries.UpdatePluginStatus(ctx, gen.UpdatePluginStatusParams{
		ID:        pgutil.UUIDToPgtype(id),
		Status:    string(status),
		ErrorLog:  pgutil.StrPtrOrNil(errorLog),
		EnabledAt: pgutil.OptTimeToPgtype(enabledAt),
	})
	return pgutil.MapErr(err, "update plugin status", plugin.ErrPluginNotFound)
}

func (r *PluginRepository) UpdateConfig(ctx context.Context, id string, config map[string]string) error {
	configJSON, err := json.Marshal(config)
	if err != nil {
		return fmt.Errorf("marshal config for update: %w", err)
	}
	err = r.queries.UpdatePluginConfig(ctx, gen.UpdatePluginConfigParams{
		ID:     pgutil.UUIDToPgtype(id),
		Config: configJSON,
	})
	return pgutil.MapErr(err, "update plugin config", plugin.ErrPluginNotFound)
}

func (r *PluginRepository) UpdatePlugin(ctx context.Context, p *plugin.Plugin) error {
	manifestJSON, err := json.Marshal(p.Manifest)
	if err != nil {
		return fmt.Errorf("marshal plugin manifest for update: %w", err)
	}

	err = r.queries.UpdatePlugin(ctx, gen.UpdatePluginParams{
		ID:          pgutil.UUIDToPgtype(p.ID),
		Name:        p.Name,
		Version:     p.Version,
		Description: pgutil.StrPtrOrNil(p.Description),
		Author:      pgutil.StrPtrOrNil(p.Author),
		License:     pgutil.StrPtrOrNil(p.License),
		SdkVersion:  pgutil.StrPtrOrNil(p.SDKVersion),
		Lang:        pgutil.StrPtrOrNil(p.Lang),
		WasmBytes:   p.WASMBytes,
		WasmHash:    pgutil.StrPtrOrNil(p.WASMHash),
		Manifest:    manifestJSON,
		Permissions: permissionsToStrings(p.Permissions),
		UpdatedAt:   pgutil.TimeToPgtype(p.UpdatedAt),
	})
	return pgutil.MapErr(err, "update plugin", plugin.ErrPluginNotFound)
}

func (r *PluginRepository) Delete(ctx context.Context, id string) error {
	err := r.queries.DeletePlugin(ctx, pgutil.UUIDToPgtype(id))
	return pgutil.MapErr(err, "delete plugin", plugin.ErrPluginNotFound)
}

func (r *PluginRepository) GetWASMByHash(ctx context.Context, hash string) ([]byte, error) {
	data, err := r.queries.GetWASMByHash(ctx, hash)
	if err != nil {
		return nil, pgutil.MapErr(err, "get WASM by hash", plugin.ErrWASMNotFound)
	}
	return data, nil
}

func (r *PluginRepository) StoreWASM(ctx context.Context, hash string, data []byte) error {
	err := r.queries.StoreWASM(ctx, gen.StoreWASMParams{
		Hash:      hash,
		Data:      data,
		SizeBytes: int64(len(data)),
	})
	return pgutil.MapErr(err, "store WASM", plugin.ErrPluginNotFound)
}

// compile-time interface check
var _ plugin.PluginRepository = (*PluginRepository)(nil)
