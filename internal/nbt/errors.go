package nbt

import "errors"

// ErrNotCompound is used when a root tag was expected to be a compound, but isn't.
var ErrNotCompound = errors.New("root tag is not a compound")
