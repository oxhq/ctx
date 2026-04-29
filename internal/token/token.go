package token

import "math"

type Tokenizer interface {
	Count(text string) int
}

type EstimateTokenizer struct {
	CharsPerToken float64
}

func (t EstimateTokenizer) Count(text string) int {
	charsPerToken := t.CharsPerToken
	if charsPerToken <= 0 {
		charsPerToken = 4.0
	}
	return int(math.Ceil(float64(len(text)) / charsPerToken))
}
