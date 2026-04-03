package aggregate

import (
	"errors"
	"time"

	"github.com/google/uuid"
)

// ErrMaxFamilyExceeded indicates the family group has reached its maximum member count.
var ErrMaxFamilyExceeded = errors.New("maximum family members exceeded")

// MemberRole distinguishes between the owner and regular members.
type MemberRole string

const (
	MemberOwner  MemberRole = "owner"
	MemberMember MemberRole = "member"
)

// FamilyMember represents a single member within a family group.
type FamilyMember struct {
	UserID   string
	Role     MemberRole
	Nickname string
	JoinedAt time.Time
}

// FamilyGroup is the aggregate root for a family sharing group.
type FamilyGroup struct {
	ID         string
	OwnerID    string
	MaxMembers int
	Members    []FamilyMember
	CreatedAt  time.Time
	UpdatedAt  time.Time
}

// NewFamilyGroup creates a new family group with the owner as the first member.
func NewFamilyGroup(ownerID string, maxMembers int, now time.Time) *FamilyGroup {
	return &FamilyGroup{
		ID:         uuid.New().String(),
		OwnerID:    ownerID,
		MaxMembers: maxMembers,
		Members: []FamilyMember{
			{
				UserID:   ownerID,
				Role:     MemberOwner,
				Nickname: "",
				JoinedAt: now,
			},
		},
		CreatedAt: now,
		UpdatedAt: now,
	}
}

// AddMember adds a new member to the family group.
// Returns an error if the group is full or the user is already a member.
func (fg *FamilyGroup) AddMember(userID, nickname string, now time.Time) error {
	if fg.HasMember(userID) {
		return errors.New("user is already a member of this family group")
	}
	if fg.IsFull() {
		return ErrMaxFamilyExceeded
	}

	fg.Members = append(fg.Members, FamilyMember{
		UserID:   userID,
		Role:     MemberMember,
		Nickname: nickname,
		JoinedAt: now,
	})
	fg.UpdatedAt = now
	return nil
}

// RemoveMember removes a member from the family group.
// The owner cannot be removed.
func (fg *FamilyGroup) RemoveMember(userID string, now time.Time) error {
	if userID == fg.OwnerID {
		return errors.New("cannot remove the owner from the family group")
	}

	for i, m := range fg.Members {
		if m.UserID == userID {
			fg.Members = append(fg.Members[:i], fg.Members[i+1:]...)
			fg.UpdatedAt = now
			return nil
		}
	}
	return errors.New("member not found in family group")
}

// IsFull reports whether the family group has reached its maximum member count.
func (fg *FamilyGroup) IsFull() bool {
	return len(fg.Members) >= fg.MaxMembers
}

// MemberCount returns the current number of members (including the owner).
func (fg *FamilyGroup) MemberCount() int {
	return len(fg.Members)
}

// HasMember reports whether the given user is a member of this group.
func (fg *FamilyGroup) HasMember(userID string) bool {
	for _, m := range fg.Members {
		if m.UserID == userID {
			return true
		}
	}
	return false
}
