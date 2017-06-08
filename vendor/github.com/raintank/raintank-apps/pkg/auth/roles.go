package auth

import (
	"errors"
	"time"
)

// Typed errors
var (
	ErrInvalidRoleType = errors.New("Invalid role type")
	ErrInvalidApiKey   = errors.New("Invalid API Key")
	ErrInvalidOrgId    = errors.New("Invalid Org Id")
)

type RoleType string

const (
	ROLE_VIEWER           RoleType = "Viewer"
	ROLE_EDITOR           RoleType = "Editor"
	ROLE_READ_ONLY_EDITOR RoleType = "Read Only Editor"
	ROLE_ADMIN            RoleType = "Admin"
)

func (r RoleType) IsValid() bool {
	return r == ROLE_VIEWER || r == ROLE_ADMIN || r == ROLE_EDITOR || r == ROLE_READ_ONLY_EDITOR
}

type SignedInUser struct {
	Id        int64     `json:"id"`
	OrgName   string    `json:"orgName"`
	OrgId     int64     `json:"orgId"`
	OrgSlug   string    `json:"orgSlug"`
	Name      string    `json:"name"`
	Role      RoleType  `json:"role"`
	CreatedAt time.Time `json:"createAt"`
	IsAdmin   bool      `json:"-"`
	key       string
}
