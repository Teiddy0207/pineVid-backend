package entity

import "errors"

var (
	ErrUserNotFound       = errors.New("user not found")
	ErrUserAlreadyExists  = errors.New("user already exists")
	ErrInvalidCredentials = errors.New("invalid credentials")
	ErrUserBanned         = errors.New("this account has been banned")
	ErrCannotFollowSelf   = errors.New("cannot follow your own channel")
	ErrTaskNotFound       = errors.New("task not found")
	ErrTaskForbidden      = errors.New("task does not belong to user")
	ErrInvalidTransition  = errors.New("invalid status transition")
)
