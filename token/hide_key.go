package token

// MaskRight is used to mask a string
func MaskRight(s string) string {
	rs := []rune(s)
	for i := len(rs) - 1; i > 3; i-- {
		rs[i] = '*'
	}
	return string(rs)
}
