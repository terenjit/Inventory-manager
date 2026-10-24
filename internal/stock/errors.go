package stock

import "inventory-manager/internal/apperr"

var ErrInsufficientStock = apperr.Conflict("insufficient stock")
