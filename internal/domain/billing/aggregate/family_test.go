package aggregate

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// newTestFamilyGroup is a test helper that creates a FamilyGroup and fails
// the test immediately if construction returns an error.
func newTestFamilyGroup(t *testing.T, ownerID string, maxMembers int) *FamilyGroup {
	t.Helper()
	fg, err := NewFamilyGroup(ownerID, maxMembers, time.Now())
	require.NoError(t, err)
	return fg
}

func TestNewFamilyGroup(t *testing.T) {
	fg, err := NewFamilyGroup("owner-1", 5, time.Now())
	require.NoError(t, err)

	assert.NotEmpty(t, fg.ID)
	assert.Equal(t, "owner-1", fg.OwnerID)
	assert.Equal(t, 5, fg.MaxMembers)
	assert.Len(t, fg.Members, 1)
	assert.Equal(t, "owner-1", fg.Members[0].UserID)
	assert.Equal(t, MemberOwner, fg.Members[0].Role)
	assert.False(t, fg.CreatedAt.IsZero())
	assert.False(t, fg.UpdatedAt.IsZero())

	assert.True(t, fg.HasEvents(), "NewFamilyGroup must record a creation event")
	events := fg.DomainEvents()
	require.Len(t, events, 1)
	assert.Equal(t, EventFamilyCreated, events[0].Type)
	assert.Equal(t, fg.ID, events[0].EntityID)

	payload, ok := events[0].Data.(FamilyCreatedPayload)
	require.True(t, ok)
	assert.Equal(t, fg.ID, payload.FamilyGroupID)
	assert.Equal(t, "owner-1", payload.OwnerID)
	assert.Equal(t, 5, payload.MaxMembers)
}

func TestNewFamilyGroup_Validation(t *testing.T) {
	t.Run("empty owner ID returns error", func(t *testing.T) {
		_, err := NewFamilyGroup("", 5, time.Now())

		require.Error(t, err)
		assert.ErrorContains(t, err, "owner ID must not be empty")
	})

	t.Run("maxMembers below minimum returns error", func(t *testing.T) {
		_, err := NewFamilyGroup("owner-1", 1, time.Now())

		require.Error(t, err)
		assert.ErrorContains(t, err, "max members must be at least")
	})

	t.Run("maxMembers zero returns error", func(t *testing.T) {
		_, err := NewFamilyGroup("owner-1", 0, time.Now())

		require.Error(t, err)
		assert.ErrorContains(t, err, "max members must be at least")
	})

	t.Run("maxMembers exceeds maximum returns error", func(t *testing.T) {
		_, err := NewFamilyGroup("owner-1", MaxFamilyMembers+1, time.Now())

		require.Error(t, err)
		assert.ErrorContains(t, err, "max members must not exceed")
	})

	t.Run("maxMembers at boundary succeeds", func(t *testing.T) {
		fg, err := NewFamilyGroup("owner-1", MinFamilyMembers, time.Now())

		require.NoError(t, err)
		assert.Equal(t, MinFamilyMembers, fg.MaxMembers)
	})

	t.Run("maxMembers at upper boundary succeeds", func(t *testing.T) {
		fg, err := NewFamilyGroup("owner-1", MaxFamilyMembers, time.Now())

		require.NoError(t, err)
		assert.Equal(t, MaxFamilyMembers, fg.MaxMembers)
	})
}

func TestFamilyGroup_AddMember(t *testing.T) {
	fg := newTestFamilyGroup(t, "owner-1", 5)

	err := fg.AddMember("user-2", "Alice", time.Now())

	require.NoError(t, err)
	assert.Len(t, fg.Members, 2)
	assert.Equal(t, "user-2", fg.Members[1].UserID)
	assert.Equal(t, MemberMember, fg.Members[1].Role)
	assert.Equal(t, "Alice", fg.Members[1].Nickname)
	assert.False(t, fg.Members[1].JoinedAt.IsZero())
}

func TestFamilyGroup_AddMember_MaxExceeded(t *testing.T) {
	fg := newTestFamilyGroup(t, "owner-1", 2)
	require.NoError(t, fg.AddMember("user-2", "Alice", time.Now()))

	err := fg.AddMember("user-3", "Bob", time.Now())

	require.Error(t, err)
	assert.ErrorIs(t, err, ErrMaxFamilyExceeded)
	assert.Len(t, fg.Members, 2)
}

func TestFamilyGroup_AddMember_Duplicate(t *testing.T) {
	fg := newTestFamilyGroup(t, "owner-1", 5)
	require.NoError(t, fg.AddMember("user-2", "Alice", time.Now()))

	err := fg.AddMember("user-2", "Alice Again", time.Now())

	require.Error(t, err)
	assert.ErrorContains(t, err, "already a member")
	assert.Len(t, fg.Members, 2)
}

func TestFamilyGroup_AddMember_DuplicateOwner(t *testing.T) {
	fg := newTestFamilyGroup(t, "owner-1", 5)

	err := fg.AddMember("owner-1", "Owner Again", time.Now())

	require.Error(t, err)
	assert.ErrorContains(t, err, "already a member")
}

func TestFamilyGroup_RemoveMember(t *testing.T) {
	fg := newTestFamilyGroup(t, "owner-1", 5)
	require.NoError(t, fg.AddMember("user-2", "Alice", time.Now()))
	assert.Len(t, fg.Members, 2)

	err := fg.RemoveMember("user-2", time.Now())

	require.NoError(t, err)
	assert.Len(t, fg.Members, 1)
	assert.Equal(t, "owner-1", fg.Members[0].UserID)
}

func TestFamilyGroup_RemoveOwner_Error(t *testing.T) {
	fg := newTestFamilyGroup(t, "owner-1", 5)

	err := fg.RemoveMember("owner-1", time.Now())

	require.Error(t, err)
	assert.ErrorContains(t, err, "owner")
	assert.Len(t, fg.Members, 1)
}

func TestFamilyGroup_RemoveMember_NotFound(t *testing.T) {
	fg := newTestFamilyGroup(t, "owner-1", 5)

	err := fg.RemoveMember("nonexistent", time.Now())

	require.Error(t, err)
	assert.ErrorContains(t, err, "not found")
}

func TestFamilyGroup_IsFull(t *testing.T) {
	fg := newTestFamilyGroup(t, "owner-1", 2)

	assert.False(t, fg.IsFull())

	require.NoError(t, fg.AddMember("user-2", "Alice", time.Now()))

	assert.True(t, fg.IsFull())
}

func TestFamilyGroup_MemberCount(t *testing.T) {
	fg := newTestFamilyGroup(t, "owner-1", 5)
	assert.Equal(t, 1, fg.MemberCount())

	require.NoError(t, fg.AddMember("user-2", "Alice", time.Now()))
	assert.Equal(t, 2, fg.MemberCount())

	require.NoError(t, fg.AddMember("user-3", "Bob", time.Now()))
	assert.Equal(t, 3, fg.MemberCount())
}

func TestFamilyGroup_HasMember(t *testing.T) {
	fg := newTestFamilyGroup(t, "owner-1", 5)

	assert.True(t, fg.HasMember("owner-1"))
	assert.False(t, fg.HasMember("user-2"))

	require.NoError(t, fg.AddMember("user-2", "Alice", time.Now()))
	assert.True(t, fg.HasMember("user-2"))
}

func TestFamilyGroup_RemoveAndReAdd(t *testing.T) {
	fg := newTestFamilyGroup(t, "owner-1", 5)
	require.NoError(t, fg.AddMember("user-2", "Alice", time.Now()))
	require.NoError(t, fg.RemoveMember("user-2", time.Now()))
	assert.False(t, fg.HasMember("user-2"))

	err := fg.AddMember("user-2", "Alice Returned", time.Now())

	require.NoError(t, err)
	assert.True(t, fg.HasMember("user-2"))
	assert.Equal(t, 2, fg.MemberCount())
}
