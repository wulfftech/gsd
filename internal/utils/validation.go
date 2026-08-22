package utils

import "regexp"

// Username validation regex: only lowercase a-z, digits 0-9, dots, hyphens, and at signs.
//
// The hyphen must stay last in the character class. Written as `[a-z0-9.-@]`
// the `.-@` is parsed as a RANGE from '.' (0x2E) to '@' (0x40), which both
// excluded the hyphen the error message promises and silently admitted
// '/', ':', ';', '<', '=', '>' and '?' into usernames.
var usernameRegex = regexp.MustCompile(`^[a-z0-9.@-]+$`)

// IsValidUsername reports whether username contains only lowercase letters,
// digits, dots, at signs and hyphens.
func IsValidUsername(username string) bool {
	return usernameRegex.MatchString(username)
}
