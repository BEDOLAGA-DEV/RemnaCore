package aggregate

import (
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/BEDOLAGA-DEV/RemnaCore/pkg/domainevent"
)

// MaxFamilyMembers is the upper bound for a family group's maximum member count.
const MaxFamilyMembers = 20

// MinFamilyMembers is the lower bound for a family group's maximum member count
// (owner + at least 1 member).
const MinFamilyMembers = 2

// compile-time check that FamilyGroup embeds EventRecorder
var _ interface{ HasEvents() bool } = (*FamilyGroup)(nil)

var (
	// ErrMaxFamilyExceeded indicates the family group has reached its maximum member count.
	ErrMaxFamilyExceeded = errors.New("maximum family members exceeded")
	// ErrAlreadyMember indicates the user is already part of this family group.
	ErrAlreadyMember = errors.New("user is already a member of this family group")
	// ErrCannotRemoveOwner indicates an attempt to remove the group owner.
	ErrCannotRemoveOwner = errors.New("cannot remove the owner from the family group")
	// ErrMemberNotFound indicates the user is not a member of this family group.
	ErrMemberNotFound = errors.New("member not found in family group")
)

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
// It embeds EventRecorder to accumulate domain events during mutations.
type FamilyGroup struct {
	domainevent.EventRecorder

	ID         string
	OwnerID    string
	MaxMembers int
	Members    []FamilyMember
	CreatedAt  time.Time
	UpdatedAt  time.Time
}

// NewFamilyGroup creates a new family group with the owner as the first member.
// Returns an error if the ownerID is empty, maxMembers is below MinFamilyMembers,
// or maxMembers exceeds MaxFamilyMembers.
func NewFamilyGroup(ownerID string, maxMembers int, now time.Time) (*FamilyGroup, error) {
	if ownerID == "" {
		return nil, errors.New("owner ID must not be empty")
	}
	if maxMembers < MinFamilyMembers {
		return nil, fmt.Errorf("max members must be at least %d (owner + 1 member)", MinFamilyMembers)
	}
	if maxMembers > MaxFamilyMembers {
		return nil, fmt.Errorf("max members must not exceed %d", MaxFamilyMembers)
	}

	fg := &FamilyGroup{
		ID:         uuid.Must(uuid.NewV7()).String(),
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
	fg.RecordEvent(domainevent.NewTyped(FamilyCreatedPayload{
		FamilyGroupID: fg.ID,
		OwnerID:       fg.OwnerID,
		MaxMembers:    fg.MaxMembers,
	}, now, fg.ID))
	return fg, nil
}

// AddMember adds a new member to the family group.
// Returns an error if the group is full or the user is already a member.
func (fg *FamilyGroup) AddMember(userID, nickname string, now time.Time) error {
	if fg.HasMember(userID) {
		return ErrAlreadyMember
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
	fg.RecordEvent(domainevent.NewTyped(FamilyMemberAddedPayload{
		FamilyGroupID: fg.ID,
		OwnerID:       fg.OwnerID,
		MemberID:      userID,
	}, now, fg.ID))
	return nil
}

// RemoveMember removes a member from the family group.
// The owner cannot be removed.
func (fg *FamilyGroup) RemoveMember(userID string, now time.Time) error {
	if userID == fg.OwnerID {
		return ErrCannotRemoveOwner
	}

	for i, m := range fg.Members {
		if m.UserID == userID {
			fg.Members = append(fg.Members[:i], fg.Members[i+1:]...)
			fg.UpdatedAt = now
			fg.RecordEvent(domainevent.NewTyped(FamilyMemberRemovedPayload{
				FamilyGroupID: fg.ID,
				OwnerID:       fg.OwnerID,
				MemberID:      userID,
			}, now, fg.ID))
			return nil
		}
	}
	return ErrMemberNotFound
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
