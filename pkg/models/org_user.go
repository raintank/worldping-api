package models

import (
	"errors"
)

// Typed errors
var (
	ErrInvalidRoleType     = errors.New("Invalid role type")
	ErrLastOrgAdmin        = errors.New("Cannot remove last organization admin")
	ErrOrgUserNotFound     = errors.New("Cannot find the organization user")
	ErrOrgUserAlreadyAdded = errors.New("User is already added to organization")
	ErrInvalidApiKey       = errors.New("Invalid API Key")
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
	UserId         int64
	OrgId          int64
	OrgName        string
	OrgRole        RoleType
	Login          string
	Name           string
	Email          string
	Theme          string
	ApiKeyId       int64
	IsGrafanaAdmin bool
}
