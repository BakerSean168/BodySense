package repository

import (
	"testing"

	"github.com/bodysense/api/internal/model"
	"github.com/google/uuid"
)

// windowTurn describes one synthetic interaction for planRetentionWindow.
type windowTurn struct {
	run     *uuid.UUID // nil = message without a Run
	status  string     // Run status
	pending bool       // an Ask User still waiting on the Run
	bytes   int        // content bytes of each of the two messages
}

func windowMessages(turns ...windowTurn) []retentionMessage {
	var out []retentionMessage
	seq := 0
	for _, turn := range turns {
		turnID := uuid.New()
		for range 2 {
			seq++
			m := retentionMessage{ID: uuid.New(), Seq: seq, TurnID: turnID, RunID: turn.run,
				Status: "completed", PendingInteraction: turn.pending, ContentBytes: turn.bytes, PartsBytes: 2}
			if turn.run != nil {
				status := turn.status
				m.RunStatus = &status
			}
			out = append(out, m)
		}
	}
	return out
}

func newRun() *uuid.UUID {
	id := uuid.New()
	return &id
}

// 96 content bytes = 4 + 32 = 36 estimated tokens a message, 72 a turn.
func settledTurn() windowTurn {
	return windowTurn{run: newRun(), status: "completed", bytes: 96}
}

func conversationAt(processed, contextStart, pruned int) *model.Conversation {
	return &model.Conversation{ProcessedThroughSeq: processed, ContextStartSeq: contextStart, PrunedThroughSeq: pruned}
}

func TestEstimateChatTokensIsConservative(t *testing.T) {
	cases := []struct{ content, parts, want int }{
		{0, 2, 5},   // empty text, "[]" parts
		{96, 2, 36}, // ASCII: one token per three bytes
		{97, 2, 37}, // rounds up
		{2, 300, 104},
		{3 * 100, 2, 104}, // 100 CJK characters estimate at 100 tokens
	}
	for _, c := range cases {
		if got := model.EstimateChatTokens(c.content, c.parts); got != c.want {
			t.Fatalf("EstimateChatTokens(%d,%d)=%d want %d", c.content, c.parts, got, c.want)
		}
	}
}

func TestChatContextTokenBudgetFromEnvironment(t *testing.T) {
	env := func(v string) func(string) string { return func(string) string { return v } }
	if got := model.ChatContextTokenBudget(env("")); got != model.DefaultChatContextTokenBudget {
		t.Fatalf("unset budget = %d", got)
	}
	if got := model.ChatContextTokenBudget(env(" 24000 ")); got != 24000 {
		t.Fatalf("configured budget = %d", got)
	}
	for _, bad := range []string{"abc", "-5", "1023"} {
		if got := model.ChatContextTokenBudget(env(bad)); got != model.DefaultChatContextTokenBudget {
			t.Fatalf("budget %q = %d, want default", bad, got)
		}
	}
}

func TestPlanRetentionWindowBudgetBoundaries(t *testing.T) {
	messages := windowMessages(settledTurn(), settledTurn(), settledTurn())
	cases := []struct {
		name          string
		budget        int
		wantP, wantC  int
		retainedLimit int
	}{
		{"exactly at budget keeps everything", 216, 6, 1, 216},
		{"one token over drops one whole interaction", 215, 6, 3, 144},
		{"room for one interaction", 72, 6, 5, 72},
		{"newest interaction is kept even above budget", 10, 6, 5, 72},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			p, cs := planRetentionWindow(conversationAt(0, 1, 0), messages, c.budget)
			if p != c.wantP || cs != c.wantC {
				t.Fatalf("P=%d C=%d, want P=%d C=%d", p, cs, c.wantP, c.wantC)
			}
			retained := 0
			for _, m := range messages {
				if m.Seq >= cs {
					retained += m.tokens()
				}
			}
			if retained != c.retainedLimit {
				t.Fatalf("retained %d tokens, want %d", retained, c.retainedLimit)
			}
		})
	}
}

func TestPlanRetentionWindowNeverPassesAnUnansweredAskUser(t *testing.T) {
	// The Ask User Run sits in the middle; later turns cannot exist while it
	// waits, so it is the newest interaction, and even a budget it alone
	// overflows keeps it.
	waiting := windowTurn{run: newRun(), status: "waiting_user", pending: true, bytes: 96}
	messages := windowMessages(settledTurn(), settledTurn(), waiting)
	p, c := planRetentionWindow(conversationAt(0, 1, 0), messages, 10)
	if p != 4 || c != 5 {
		t.Fatalf("P=%d C=%d, want P=4 C=5", p, c)
	}
	// Even with forced unrelated settled turns after it, P and C stop in
	// front of the unanswered Run.
	messages = windowMessages(settledTurn(), waiting, settledTurn(), settledTurn())
	p, c = planRetentionWindow(conversationAt(0, 1, 0), messages, 10)
	if p != 2 || c != 3 {
		t.Fatalf("P=%d C=%d, want P=2 C=3", p, c)
	}
}

func TestPlanRetentionWindowStopsAtStreamingAndLiveRuns(t *testing.T) {
	live := windowTurn{run: newRun(), status: "running", bytes: 96}
	messages := windowMessages(settledTurn(), settledTurn(), live)
	p, c := planRetentionWindow(conversationAt(0, 1, 0), messages, 72)
	if p != 4 || c != 5 {
		t.Fatalf("live run: P=%d C=%d", p, c)
	}
	messages = windowMessages(settledTurn(), settledTurn(), settledTurn())
	messages[3].Status = "streaming"
	p, c = planRetentionWindow(conversationAt(0, 1, 0), messages, 72)
	if p != 2 || c != 3 {
		t.Fatalf("streaming reply: P=%d C=%d", p, c)
	}
}

func TestPlanRetentionWindowNeverSplitsARun(t *testing.T) {
	shared := newRun()
	first := windowTurn{run: shared, status: "completed", bytes: 96}
	messages := windowMessages(first, first, settledTurn())
	p, c := planRetentionWindow(conversationAt(0, 1, 0), messages, 144)
	// Dropping only the first turn would split the shared Run; the next
	// boundary that splits nothing is after both of its turns.
	if p != 6 || c != 5 {
		t.Fatalf("P=%d C=%d, want P=6 C=5", p, c)
	}
	if runBoundary(messages, 2) {
		t.Fatal("seq 2 splits the shared run")
	}
}

func TestPlanRetentionWindowIsMonotonicAndIdempotent(t *testing.T) {
	messages := windowMessages(settledTurn(), settledTurn(), settledTurn())
	p, c := planRetentionWindow(conversationAt(6, 5, 0), messages, 1_000_000)
	if p != 6 || c != 5 {
		t.Fatalf("a larger budget moved C back: P=%d C=%d", p, c)
	}
	p1, c1 := planRetentionWindow(conversationAt(0, 1, 0), messages, 100)
	p2, c2 := planRetentionWindow(conversationAt(p1, c1, 0), messages, 100)
	if p1 != p2 || c1 != c2 {
		t.Fatalf("second plan moved: (%d,%d) -> (%d,%d)", p1, c1, p2, c2)
	}
}

func TestPlanRetentionWindowIgnoresPrunedAndLeftMessages(t *testing.T) {
	// Messages before C no longer count toward the budget.
	messages := windowMessages(settledTurn(), settledTurn(), settledTurn(), settledTurn())
	p, c := planRetentionWindow(conversationAt(4, 5, 2), messages[2:], 144)
	if p != 8 || c != 5 {
		t.Fatalf("P=%d C=%d, want P=8 C=5", p, c)
	}
}
