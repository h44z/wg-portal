package domain

import "errors"

var ErrNotFound = errors.New("record not found")
var ErrNotUnique = errors.New("record not unique")
