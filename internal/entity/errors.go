package entity

import "errors"

var (
	ErrUserNotFound       = errors.New("user not found")
	ErrUserAlreadyExists  = errors.New("user already exists")
	ErrInvalidCredentials = errors.New("invalid credentials")
	ErrUserBanned         = errors.New("this account has been banned")
	ErrCannotFollowSelf   = errors.New("cannot follow your own channel")
	ErrInvalidTransition  = errors.New("invalid status transition")
)
