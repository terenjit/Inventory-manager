package auth

import "inventory-manager/internal/apperr"

var (
	ErrInvalidCredentials = apperr.Unauthorized("invalid username or password")
	ErrUserNotActive      = apperr.Unauthorized("account is deactivated")
	ErrUsernameTaken      = apperr.Conflict("username already taken")
	ErrCashierNeedsStore  = apperr.BadRequest("cashier must be assigned to a store")
	ErrStoreNotFound      = apperr.NotFound("store not found")
)
