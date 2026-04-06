package tokens

// Estimate returns the estimated token count for the given character count.
// Uses the chars/4 heuristic: if charCount%4 >= 2, rounds up; otherwise truncates.
func Estimate(charCount int) int {
	result := charCount / 4
	if charCount%4 >= 2 {
		result++
	}
	return result
}
