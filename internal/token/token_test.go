package token

import "testing"

func TestEstimateTokenizerCountsByCeilingCharsPerToken(t *testing.T) {
	tok := EstimateTokenizer{CharsPerToken: 4}

	if got := tok.Count("12345"); got != 2 {
		t.Fatalf("expected 2 tokens, got %d", got)
	}
}

func TestEstimateTokenizerDefaultsInvalidCharsPerToken(t *testing.T) {
	tok := EstimateTokenizer{}

	if got := tok.Count("1234"); got != 1 {
		t.Fatalf("expected default tokenizer to count 1 token, got %d", got)
	}
}
