package postgres

import (
	"context"
	"errors"
	"testing"
)

// stubRow implements pgx.Row over a single bool, for testing the boot
// assertion without a database.
type stubRow struct {
	bypasses bool
	scanErr  error
}

func (r stubRow) Scan(dest ...any) error {
	if r.scanErr != nil {
		return r.scanErr
	}
	*(dest[0].(*bool)) = r.bypasses
	return nil
}

type stubQuerier struct{ row stubRow }

func (q stubQuerier) QueryRow(_ context.Context, _ string, _ ...any) pgxRow {
	return q.row
}

func TestAssertNonBypassRLSRole(t *testing.T) {
	if err := assertNonBypassRLSRole(context.Background(), stubQuerier{row: stubRow{bypasses: false}}); err != nil {
		t.Fatalf("non-bypassing role should pass, got %v", err)
	}
	if err := assertNonBypassRLSRole(context.Background(), stubQuerier{row: stubRow{bypasses: true}}); err == nil {
		t.Fatal("superuser/BYPASSRLS role must fail boot")
	}
	wantErr := errors.New("boom")
	if err := assertNonBypassRLSRole(context.Background(), stubQuerier{row: stubRow{scanErr: wantErr}}); err == nil {
		t.Fatal("scan error must propagate")
	}
}
