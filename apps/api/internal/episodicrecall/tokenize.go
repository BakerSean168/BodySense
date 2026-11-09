package episodicrecall

import (
	"sort"
	"strings"
	"unicode"
	"unicode/utf8"
)

// Chinese text has no whitespace word boundaries. PostgreSQL in this
// deployment has no Chinese segmenter (see
// docs/architecture/episodic-s2-benchmark-recall-eval.md §6), so the S4
// keyword path matches character bigrams instead of lexemes. This file is the
// single tokenizer used by both the runtime recall path and the offline
// benchmark, so "same measurement口径" is structural rather than a promise.

// MaxQueryBigrams bounds the token set per query, exactly like the baseline
// control (`bigrams` in benchmarks/episodic/recall.go).
const MaxQueryBigrams = 64

// queryStopwords are conversation filler and time references that carry no
// subject. The list is the baseline evaluation's list; changing it changes the
// measured recall and must be re-validated with the S4 fixture.
var queryStopwords = mustSortedStopwords([]string{
	"你还记得", "我提到的", "我之前说过的", "我说了什么", "帮我回忆一下",
	"回忆一下", "还有印象", "有印象", "是不是", "我之前的", "我之前", "说过的",
	"我说过的", "讲过", "聊到", "提到的", "提到过", "提到", "说过", "记得",
	"上次", "之前", "那次", "的时候", "时候", "后来", "怎么样", "什么",
	"细节", "事情", "事", "吗", "呢", "的", "了", "我", "你", "这个", "那个",
	"这段", "这段时间", "前阵子", "上个月", "两三个月前", "前几个月",
	"三个月前", "四个多月前", "半年前", "半年多前", "年初", "前一阵",
	"去年这个时候", "去年冬天", "去年", "关于", "我们", "可以", "以后", "请", "帮",
})

func mustSortedStopwords(in []string) []string {
	out := append([]string(nil), in...)
	sort.Slice(out, func(i, j int) bool {
		if len([]rune(out[i])) != len([]rune(out[j])) {
			return len([]rune(out[i])) > len([]rune(out[j]))
		}
		return out[i] < out[j]
	})
	return out
}

// ExtractTerms removes filler phrases, punctuation and whitespace, returning
// the remaining subject terms. Terms shorter than two runes are dropped.
func ExtractTerms(query string) []string {
	s := query
	for _, w := range queryStopwords {
		s = strings.ReplaceAll(s, w, "|")
	}
	s = strings.Map(func(r rune) rune {
		if unicode.IsPunct(r) || unicode.IsSpace(r) {
			return '|'
		}
		return r
	}, s)
	seen := map[string]bool{}
	var out []string
	for _, part := range strings.Split(s, "|") {
		part = strings.TrimSpace(part)
		if utf8.RuneCountInString(part) < 2 || seen[part] {
			continue
		}
		seen[part] = true
		out = append(out, part)
	}
	if len(out) == 0 {
		if trimmed := strings.TrimSpace(query); trimmed != "" {
			out = append(out, trimmed)
		}
	}
	return out
}

// QueryTokens is the runtime entry point: extract subject terms, then expand
// each into overlapping character bigrams, deduplicated and bounded.
func QueryTokens(query string) []string {
	return Bigrams(ExtractTerms(query))
}

// Bigrams expands terms into unique overlapping character bigrams.
func Bigrams(terms []string) []string {
	seen := map[string]bool{}
	var out []string
	for _, term := range terms {
		r := []rune(term)
		for i := 0; i+1 < len(r); i++ {
			b := string(r[i : i+2])
			if seen[b] {
				continue
			}
			seen[b] = true
			out = append(out, b)
		}
	}
	if len(out) > MaxQueryBigrams {
		out = out[:MaxQueryBigrams]
	}
	return out
}

// Score is the relevance measure shared with the benchmark: the number of
// distinct query bigrams that occur verbatim in the note text. It is a lexical
// overlap count, not a normalized similarity, and it never rewrites text.
func Score(noteText string, tokens []string) int {
	matched := 0
	for _, token := range tokens {
		if strings.Contains(noteText, token) {
			matched++
		}
	}
	return matched
}
