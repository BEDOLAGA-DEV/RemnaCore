package health

import "testing"

func TestAggregate(t *testing.T) {
	tests := []struct {
		name   string
		checks []ComponentCheck
		want   Status
	}{
		{
			name:   "empty checks",
			checks: nil,
			want:   StatusHealthy,
		},
		{
			name: "all healthy",
			checks: []ComponentCheck{
				{Name: "a", Status: StatusHealthy},
				{Name: "b", Status: StatusHealthy},
			},
			want: StatusHealthy,
		},
		{
			name: "one degraded",
			checks: []ComponentCheck{
				{Name: "a", Status: StatusHealthy},
				{Name: "b", Status: StatusDegraded, Message: "backlog high"},
			},
			want: StatusDegraded,
		},
		{
			name: "one unhealthy overrides degraded",
			checks: []ComponentCheck{
				{Name: "a", Status: StatusDegraded},
				{Name: "b", Status: StatusUnhealthy},
				{Name: "c", Status: StatusHealthy},
			},
			want: StatusUnhealthy,
		},
		{
			name: "all unhealthy",
			checks: []ComponentCheck{
				{Name: "a", Status: StatusUnhealthy},
				{Name: "b", Status: StatusUnhealthy},
			},
			want: StatusUnhealthy,
		},
		{
			name: "unhealthy short-circuits on first",
			checks: []ComponentCheck{
				{Name: "a", Status: StatusUnhealthy},
				{Name: "b", Status: StatusHealthy},
			},
			want: StatusUnhealthy,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Aggregate(tt.checks)
			if got != tt.want {
				t.Errorf("Aggregate() = %q, want %q", got, tt.want)
			}
		})
	}
}
