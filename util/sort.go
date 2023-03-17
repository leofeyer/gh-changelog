package util

import (
	"unicode"
	"unicode/utf8"
)

func SortCaseInsensitive(s, t string) bool {
	for {
		if len(t) == 0 {
			return false
		}

		if len(s) == 0 {
			return true
		}

		c, sizec := utf8.DecodeRuneInString(s)
		d, sized := utf8.DecodeRuneInString(t)

		lowerc := unicode.ToLower(c)
		lowerd := unicode.ToLower(d)

		if lowerc < lowerd {
			return true
		}

		if lowerc > lowerd {
			return false
		}

		s = s[sizec:]
		t = t[sized:]
	}
}
