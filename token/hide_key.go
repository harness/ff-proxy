package token

func MaskRight(s string) string {
	rs := []rune(s)
	for i := len(rs) - 1; i > 3; i-- {
		rs[i] = '*'
	}
	return string(rs)
}
