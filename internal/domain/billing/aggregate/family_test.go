package aggregate

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewFamilyGroup(t *testing.T) {
	fg := NewFamilyGroup("owner-1", 5, time.Now())

	assert.NotEmpty(t, fg.ID)
	assert.Equal(t, "owner-1", fg.OwnerID)
	assert.Equal(t, 5, fg.MaxMembers)
	assert.Len(t, fg.Members, 1)
	assert.Equal(t, "owner-1", fg.Members[0].UserID)
	assert.Equal(t, MemberOwner, fg.Members[0].Role)
	assert.False(t, fg.CreatedAt.IsZero())
	assert.False(t, fg.UpdatedAt.IsZero())
}

func TestFamilyGroup_AddMember(t *testing.T) {
	fg := NewFamilyGroup("owner-1", 5, time.Now())

	err := fg.AddMember("user-2", "Alice", time.Now())

	require.NoError(t, err)
	assert.Len(t, fg.Members, 2)
	assert.Equal(t, "user-2", fg.Members[1].UserID)
	assert.Equal(t, MemberMember, fg.Members[1].Role)
	assert.Equal(t, "Alice", fg.Members[1].Nickname)
	assert.False(t, fg.Members[1].JoinedAt.IsZero())
}

func TestFamilyGroup_AddMember_MaxExceeded(t *testing.T) {
	fg := NewFamilyGroup("owner-1", 2, time.Now())
	require.NoError(t, fg.AddMember("user-2", "Alice", time.Now()))

	err := fg.AddMember("user-3", "Bob", time.Now())

	require.Error(t, err)
	assert.ErrorIs(t, err, ErrMaxFamilyExceeded)
	assert.Len(t, fg.Members, 2)
}

func TestFamilyGroup_AddMember_Duplicate(t *testing.T) {
	fg := NewFamilyGroup("owner-1", 5, time.Now())
	require.NoError(t, fg.AddMember("user-2", "Alice", time.Now()))

	err := fg.AddMember("user-2", "Alice Again", time.Now())

	require.Error(t, err)
	assert.ErrorContains(t, err, "already a member")
	assert.Len(t, fg.Members, 2)
}

func TestFamilyGroup_AddMember_DuplicateOwner(t *testing.T) {
	fg := NewFamilyGroup("owner-1", 5, time.Now())

	err := fg.AddMember("owner-1", "Owner Again", time.Now())

	require.Error(t, err)
	assert.ErrorContains(t, err, "already a member")
}

func TestFamilyGroup_RemoveMember(t *testing.T) {
	fg := NewFamilyGroup("owner-1", 5, time.Now())
	require.NoError(t, fg.AddMember("user-2", "Alice", time.Now()))
	assert.Len(t, fg.Members, 2)

	err := fg.RemoveMember("user-2", time.Now())

	require.NoError(t, err)
	assert.Len(t, fg.Members, 1)
	assert.Equal(t, "owner-1", fg.Members[0].UserID)
}

func TestFamilyGroup_RemoveOwner_Error(t *testing.T) {
	fg := NewFamilyGroup("owner-1", 5, time.Now())

	err := fg.RemoveMember("owner-1", time.Now())

	require.Error(t, err)
	assert.ErrorContains(t, err, "owner")
	assert.Len(t, fg.Members, 1)
}

func TestFamilyGroup_RemoveMember_NotFound(t *testing.T) {
	fg := NewFamilyGroup("owner-1", 5, time.Now())

	err := fg.RemoveMember("nonexistent", time.Now())

	require.Error(t, err)
	assert.ErrorContains(t, err, "not found")
}

func TestFamilyGroup_IsFull(t *testing.T) {
	fg := NewFamilyGroup("owner-1", 2, time.Now())

	assert.False(t, fg.IsFull())

	require.NoError(t, fg.AddMember("user-2", "Alice", time.Now()))

	assert.True(t, fg.IsFull())
}

func TestFamilyGroup_MemberCount(t *testing.T) {
	fg := NewFamilyGroup("owner-1", 5, time.Now())
	assert.Equal(t, 1, fg.MemberCount())

	require.NoError(t, fg.AddMember("user-2", "Alice", time.Now()))
	assert.Equal(t, 2, fg.MemberCount())

	require.NoError(t, fg.AddMember("user-3", "Bob", time.Now()))
	assert.Equal(t, 3, fg.MemberCount())
}

func TestFamilyGroup_HasMember(t *testing.T) {
	fg := NewFamilyGroup("owner-1", 5, time.Now())

	assert.True(t, fg.HasMember("owner-1"))
	assert.False(t, fg.HasMember("user-2"))

	require.NoError(t, fg.AddMember("user-2", "Alice", time.Now()))
	assert.True(t, fg.HasMember("user-2"))
}

func TestFamilyGroup_RemoveAndReAdd(t *testing.T) {
	fg := NewFamilyGroup("owner-1", 5, time.Now())
	require.NoError(t, fg.AddMember("user-2", "Alice", time.Now()))
	require.NoError(t, fg.RemoveMember("user-2", time.Now()))
	assert.False(t, fg.HasMember("user-2"))

	err := fg.AddMember("user-2", "Alice Returned", time.Now())

	require.NoError(t, err)
	assert.True(t, fg.HasMember("user-2"))
	assert.Equal(t, 2, fg.MemberCount())
}
