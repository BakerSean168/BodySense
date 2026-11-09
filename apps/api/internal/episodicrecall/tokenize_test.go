package episodicrecall

import (
	"reflect"
	"testing"
)

func TestQueryTokensMatchesBaselineTokenizer(t *testing.T) {
	cases := []struct {
		query string
		want  []string
	}{
		{"你还记得我上次说过的早市吗", []string{"早市"}},
		{"膝盖疼", []string{"膝盖", "盖疼"}},
		{"", nil},
		{"吗呢的", []string{"吗呢", "呢的"}},
	}
	for _, tc := range cases {
		got := QueryTokens(tc.query)
		if !reflect.DeepEqual(got, tc.want) {
			t.Fatalf("QueryTokens(%q) = %v, want %v", tc.query, got, tc.want)
		}
	}
}

func TestBigramsDeduplicatesAndCaps(t *testing.T) {
	got := Bigrams([]string{"早市", "早市", "去了早市"})
	want := []string{"早市", "去了", "了早"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("Bigrams = %v, want %v", got, want)
	}

	long := make([]rune, 0, MaxQueryBigrams*2)
	for i := 0; i < MaxQueryBigrams*2; i++ {
		long = append(long, rune('一'+i))
	}
	if n := len(Bigrams([]string{string(long)})); n != MaxQueryBigrams {
		t.Fatalf("Bigrams cap = %d, want %d", n, MaxQueryBigrams)
	}
}

func TestScoreCountsDistinctOverlappingBigrams(t *testing.T) {
	tokens := QueryTokens("膝盖 肩膀 手腕")
	note := "用户说膝盖和肩膀都不舒服。"
	if got := Score(note, tokens); got != 2 {
		t.Fatalf("Score = %d, want 2 (膝盖, 肩膀)", got)
	}
	if got := Score("完全无关的内容", tokens); got != 0 {
		t.Fatalf("Score of unrelated note = %d, want 0", got)
	}
}

func TestEpisodicRecallEnabledFromEnv(t *testing.T) {
	cases := map[string]bool{
		"":      false,
		"false": false,
		"0":     false,
		"true":  true,
		"TRUE":  true,
		"1":     true,
		"yes":   false,
	}
	for value, want := range cases {
		got := EpisodicRecallEnabledFromEnv(func(string) string { return value })
		if got != want {
			t.Fatalf("env %q => %v, want %v", value, got, want)
		}
	}
}
