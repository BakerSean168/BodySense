package main

import (
	"fmt"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"
)

// This command is an offline measurement harness. It never calls a model, a
// remote provider or the application server. Every user, conversation and
// message below is generated from a fixed seed; no real user data is read or
// written. The runtime retention caps and note admission code are untouched.

const (
	defaultSeed     = uint64(20261105)
	defaultUsers    = 8
	defaultWeeks    = 52
	segmentsPerWeek = 3
	benchNowISO     = "2026-11-05T00:00:00Z"
	corpusVersion   = "episodic-bench-corpus-v1"
)

var benchNamespace = uuid.MustParse("3f0f7e2c-1b1a-5a6d-9c7e-2a4b8d6e0f11")

type segmentKind int

const (
	kindLife segmentKind = iota
	kindStyle
	kindClinical
)

func (k segmentKind) String() string {
	switch k {
	case kindLife:
		return "life_narrative"
	case kindStyle:
		return "style_preference"
	default:
		return "clinical_statement"
	}
}

// noteEligible mirrors S2-A: only non-clinical, text-only windows may become
// a shadow note. Clinical segments are counted as deliberate generation gaps.
func (k segmentKind) noteEligible() bool { return k != kindClinical }

// anchor is the unique, human-reviewable subject of one note. Queries are
// derived from Term and Alias so that a paraphrase has a known target.
type anchor struct {
	ID      string
	Kind    segmentKind
	Term    string
	Alias   string
	Place   string // life: place/venue
	Action  string // life: activity performed there
	Detail  string // life: what happened
	Request string // style: the preference itself
}

var anchors = []anchor{
	{ID: "a01", Kind: kindLife, Term: "滨江公园", Alias: "江畔公园", Place: "滨江公园", Action: "慢跑", Detail: "膝盖外侧有点发酸"},
	{ID: "a02", Kind: kindLife, Term: "城南图书馆", Alias: "南边的图书馆", Place: "城南图书馆", Action: "翻城市更新方面的书", Detail: "一坐就是一下午"},
	{ID: "a03", Kind: kindLife, Term: "老街菜市场", Alias: "旧城那个菜场", Place: "老街菜市场", Action: "买春笋", Detail: "和摊主聊了很久"},
	{ID: "a04", Kind: kindLife, Term: "西山步道", Alias: "西山的小路", Place: "西山步道", Action: "徒步", Detail: "走到半山腰遇到阵雨"},
	{ID: "a05", Kind: kindLife, Term: "湖心亭", Alias: "湖中间的亭子", Place: "湖心亭", Action: "坐着发呆", Detail: "看了很久的鸭子"},
	{ID: "a06", Kind: kindLife, Term: "城北体育馆", Alias: "北边的体育中心", Place: "城北体育馆", Action: "打羽毛球", Detail: "手腕有点累"},
	{ID: "a07", Kind: kindLife, Term: "望江楼", Alias: "江边那座楼", Place: "望江楼", Action: "看日落", Detail: "顺手拍了几张照片"},
	{ID: "a08", Kind: kindLife, Term: "青石巷", Alias: "石板路那条巷子", Place: "青石巷", Action: "喝咖啡", Detail: "遇到一只橘猫"},
	{ID: "a09", Kind: kindLife, Term: "梧桐大道", Alias: "梧桐树那条路", Place: "梧桐大道", Action: "骑车通勤", Detail: "落叶铺了一地"},
	{ID: "a10", Kind: kindLife, Term: "南湖湿地", Alias: "南边那片湿地", Place: "南湖湿地", Action: "观鸟", Detail: "看到了白鹭"},
	{ID: "a11", Kind: kindLife, Term: "半程马拉松", Alias: "半马", Place: "半程马拉松", Action: "备赛", Detail: "配速比上次稳"},
	{ID: "a12", Kind: kindLife, Term: "普拉提", Alias: "垫上训练", Place: "普拉提课", Action: "练核心", Detail: "终于找到发力的感觉"},
	{ID: "a13", Kind: kindLife, Term: "陶艺课", Alias: "做陶的课", Place: "陶艺课", Action: "拉坯", Detail: "做歪了一个杯子"},
	{ID: "a14", Kind: kindLife, Term: "社区合唱团", Alias: "小区合唱队", Place: "社区合唱团", Action: "排练", Detail: "唱到高音有点紧"},
	{ID: "a15", Kind: kindLife, Term: "露天电影", Alias: "户外电影", Place: "露天电影", Action: "看了一场老片", Detail: "坐在地上有点凉"},
	{ID: "a16", Kind: kindLife, Term: "旧书店", Alias: "二手书店", Place: "旧书店", Action: "淘书", Detail: "找到一本旧版地图册"},
	{ID: "a17", Kind: kindLife, Term: "早市", Alias: "清晨的集市", Place: "早市", Action: "买菜", Detail: "人比想象中多"},
	{ID: "a18", Kind: kindLife, Term: "江边骑行", Alias: "沿江骑车", Place: "江边绿道", Action: "骑行", Detail: "逆风骑得很吃力"},
	{ID: "a19", Kind: kindLife, Term: "山间露营", Alias: "山里扎营", Place: "山间营地", Action: "露营", Detail: "夜里听到虫鸣"},
	{ID: "a20", Kind: kindLife, Term: "周末市集", Alias: "周末集市", Place: "周末市集", Action: "逛摊", Detail: "买了个手工木勺"},
	{ID: "a21", Kind: kindLife, Term: "老城墙", Alias: "旧城墙", Place: "老城墙", Action: "散步", Detail: "墙缝里长了草"},
	{ID: "a22", Kind: kindLife, Term: "竹林小径", Alias: "竹林小路", Place: "竹林小径", Action: "散步", Detail: "空气里有竹叶味"},
	{ID: "a23", Kind: kindLife, Term: "城西植物园", Alias: "西边植物园", Place: "城西植物园", Action: "看花", Detail: "温室里有点闷"},
	{ID: "a24", Kind: kindLife, Term: "天台花园", Alias: "楼顶花园", Place: "天台花园", Action: "浇花", Detail: "薄荷长疯了"},
	{ID: "a25", Kind: kindLife, Term: "河边钓点", Alias: "河边钓鱼的地方", Place: "河边钓点", Action: "钓鱼", Detail: "一上午没动静"},
	{ID: "a26", Kind: kindLife, Term: "老电影院", Alias: "旧影院", Place: "老电影院", Action: "看老片", Detail: "座椅有点硬"},
	{ID: "a27", Kind: kindLife, Term: "城东花市", Alias: "东边花鸟市场", Place: "城东花市", Action: "买绿植", Detail: "挑了一盆龟背竹"},
	{ID: "a28", Kind: kindLife, Term: "夜跑路线", Alias: "晚上跑步的路线", Place: "沿河步道", Action: "夜跑", Detail: "路灯有点暗"},
	{ID: "a29", Kind: kindLife, Term: "社区球场", Alias: "小区球场", Place: "社区球场", Action: "打球", Detail: "遇到几个中学生"},
	{ID: "a30", Kind: kindLife, Term: "山顶观景台", Alias: "山顶的观景平台", Place: "山顶观景台", Action: "看夜景", Detail: "风很大"},
	{ID: "a31", Kind: kindStyle, Term: "先给结论", Alias: "结论放在最前面", Request: "回答时先给结论，再补充原因"},
	{ID: "a32", Kind: kindStyle, Term: "别用敬语", Alias: "不用客气称呼", Request: "不要用「您」这类敬语称呼"},
	{ID: "a33", Kind: kindStyle, Term: "用表格对比", Alias: "表格形式对照", Request: "需要对比的时候用表格"},
	{ID: "a34", Kind: kindStyle, Term: "举生活例子", Alias: "打个生活里的比方", Request: "解释概念时举个生活里的例子"},
	{ID: "a35", Kind: kindStyle, Term: "少用英文缩写", Alias: "别堆英文简称", Request: "尽量少用英文缩写"},
	{ID: "a36", Kind: kindStyle, Term: "每次只问一个问题", Alias: "一次只问一件事", Request: "每次只问一个问题"},
	{ID: "a37", Kind: kindStyle, Term: "回复短一点", Alias: "回答简短些", Request: "回复尽量短一点"},
	{ID: "a38", Kind: kindStyle, Term: "用要点列表", Alias: "列成条目", Request: "复杂内容用要点列表"},
	{ID: "a39", Kind: kindStyle, Term: "不要重复我的话", Alias: "别复述我的原话", Request: "不要重复我说过的话"},
	{ID: "a40", Kind: kindStyle, Term: "先问再建议", Alias: "先确认再给建议", Request: "给建议前先确认情况"},
	{ID: "a41", Kind: kindStyle, Term: "数字加单位", Alias: "数值带上单位", Request: "说数字时带上单位"},
	{ID: "a42", Kind: kindStyle, Term: "避免夸张词", Alias: "别用夸张的说法", Request: "避免「超级」「巨」这类夸张词"},
	{ID: "a43", Kind: kindStyle, Term: "用中文术语", Alias: "术语用中文", Request: "专业术语尽量用中文"},
	{ID: "a44", Kind: kindStyle, Term: "直接说风险", Alias: "风险讲直白", Request: "有风险就直接说"},
	{ID: "a45", Kind: kindStyle, Term: "给两套方案", Alias: "提供两个选择", Request: "给方案时给两套备选"},
	{ID: "a46", Kind: kindStyle, Term: "结尾给下一步", Alias: "最后说下一步做什么", Request: "结尾告诉我下一步做什么"},
	{ID: "a47", Kind: kindStyle, Term: "别一次给太多选项", Alias: "选项别太多", Request: "一次别给太多选项"},
	{ID: "a48", Kind: kindStyle, Term: "用第二人称", Alias: "直接说「你」", Request: "用第二人称直接称呼"},
	{ID: "a49", Kind: kindStyle, Term: "别用感叹号", Alias: "不要感叹语气", Request: "回复里别用感叹号"},
	{ID: "a50", Kind: kindStyle, Term: "保留我的原词", Alias: "沿用我自己的说法", Request: "复述时保留我原来的说法"},
}

type syntheticMessage struct {
	ID     uuid.UUID
	TurnID uuid.UUID
	Role   string
	Seq    int
	Text   string
	At     time.Time
}

type segment struct {
	User      *corpusUser
	Week      int
	Kind      segmentKind
	SeqFrom   int
	SeqTo     int
	Messages  []syntheticMessage
	NoteID    uuid.UUID
	NoteText  string
	Anchor    *anchor
	At        time.Time
	Eligible  bool
	TargetLen int
}

type corpusUser struct {
	Index          int
	ID             uuid.UUID
	ConversationID uuid.UUID
	Verbose        bool
	Segments       []*segment
}

type corpus struct {
	Version   string
	Seed      uint64
	Weeks     int
	Users     []*corpusUser
	Segments  []*segment
	Now       time.Time
	StartDate time.Time
}

func deterministicUUID(parts ...string) uuid.UUID {
	return uuid.NewSHA1(benchNamespace, []byte(strings.Join(parts, "|")))
}

// rng is a small splitmix64 generator so results do not depend on the Go
// release's math/rand implementation.
type rng struct{ state uint64 }

func newRNG(seed uint64) *rng { return &rng{state: seed} }

func (r *rng) next() uint64 {
	r.state += 0x9e3779b97f4a7c15
	z := r.state
	z = (z ^ (z >> 30)) * 0xbf58476d1ce4e5b9
	z = (z ^ (z >> 27)) * 0x94d049bb133111eb
	return z ^ (z >> 31)
}

func (r *rng) intn(n int) int { return int(r.next() % uint64(n)) }

func (r *rng) pick(pool []string) string { return pool[r.intn(len(pool))] }

var (
	lifeWeather  = []string{"那天有点阴", "那天很晒", "那天起了风", "那天下午下过一阵小雨", "那天难得放晴"}
	lifeFeeling  = []string{"心情松了一些", "觉得比预想的累", "对这类活动更有兴趣了", "没什么特别的感觉", "回程路上一直在想别的事"}
	lifeCompany  = []string{"是临时起意", "提前查了路线", "约了同事一起", "出门比平时早", "一个人去的"}
	lifeAside    = []string{"最近工作节奏有点快", "这几天睡得比平时晚", "家里在收拾东西", "周末基本没安排别的", "通勤时间变长了一些"}
	clinicalA    = []string{"膝盖上下楼有点不舒服", "早上起床腰有点僵", "嗓子干了两天", "肩膀酸得抬不起来", "晚上总是醒"}
	clinicalB    = []string{"持续了大概一周", "休息后会好一些", "没有吃特别的药", "这两天稍微缓解", "想先观察看看"}
	styleGeneric = []string{"尽量说清楚一点", "别太长", "先听我说完", "不用每次都总结", "少给几个选项"}
)

func buildCorpus(seed uint64, users, weeks int) *corpus {
	start := time.Date(2025, time.November, 1, 10, 0, 0, 0, time.UTC)
	now, _ := time.Parse(time.RFC3339, benchNowISO)
	c := &corpus{Version: corpusVersion, Seed: seed, Weeks: weeks, Now: now, StartDate: start}
	r := newRNG(seed)
	for u := 0; u < users; u++ {
		user := &corpusUser{
			Index:          u,
			ID:             deterministicUUID(corpusVersion, fmt.Sprintf("user:%d", u)),
			ConversationID: deterministicUUID(corpusVersion, fmt.Sprintf("conversation:%d", u)),
			Verbose:        u < users/2,
		}
		seq := 1
		for w := 0; w < weeks; w++ {
			for k := 0; k < segmentsPerWeek; k++ {
				at := start.AddDate(0, 0, w*7+k*2+1).Add(time.Duration(9+k) * time.Hour)
				kind := kindFor(u, w, k)
				seg := &segment{User: user, Week: w, Kind: kind, At: at, Eligible: kind.noteEligible()}
				seg.TargetLen = targetNoteRunes(user.Verbose, r)
				seg.SeqFrom = seq
				for m := 0; m < 4; m++ {
					msg := syntheticMessage{
						ID:     deterministicUUID(corpusVersion, fmt.Sprintf("message:%d:%d", u, seq)),
						TurnID: deterministicUUID(corpusVersion, fmt.Sprintf("turn:%d:%d", u, seq)),
						Role:   "user",
						Seq:    seq,
						At:     at.Add(time.Duration(m) * time.Minute),
					}
					if m%2 == 1 {
						msg.Role = "assistant"
					}
					msg.Text = messageText(kind, m, u, seq)
					seg.Messages = append(seg.Messages, msg)
					seq++
				}
				seg.SeqTo = seq - 1
				if seg.Eligible {
					seg.NoteID = deterministicUUID(corpusVersion, fmt.Sprintf("note:%d:%d:%d", u, seg.SeqFrom, seg.SeqTo))
					seg.NoteText = draftNote(seg, r)
				}
				user.Segments = append(user.Segments, seg)
				c.Segments = append(c.Segments, seg)
			}
		}
		c.Users = append(c.Users, user)
	}
	c.assignAnchors()
	return c
}

func kindFor(user, week, slot int) segmentKind {
	switch (week*segmentsPerWeek + slot + user) % 10 {
	case 0, 1, 2:
		return kindStyle
	case 3, 4, 5, 6:
		return kindLife
	default:
		return kindClinical
	}
}

func targetNoteRunes(verbose bool, r *rng) int {
	if verbose {
		return 340 + r.intn(81) // 340..420
	}
	return 160 + r.intn(41) // 160..200
}

func messageText(kind segmentKind, turn, user, seq int) string {
	idx := (user*7 + seq/4) % 5
	switch kind {
	case kindLife:
		if turn == 0 {
			return fmt.Sprintf("这周出去走走了，去了%s附近。", []string{"滨江", "城南", "西山", "湖心", "城北"}[idx])
		}
		if turn == 2 {
			return "回来以后还挺放松的，就是有点累。"
		}
		return "听起来不错，回来休息一下。"
	case kindStyle:
		if turn == 0 {
			return fmt.Sprintf("以后%s，可以吗？", styleGeneric[idx])
		}
		if turn == 2 {
			return "行，那就这样说定了。"
		}
		return "可以，我记下了。"
	default:
		if turn == 0 {
			return fmt.Sprintf("最近%s。", clinicalA[idx])
		}
		if turn == 2 {
			return fmt.Sprintf("%s。", clinicalB[idx])
		}
		return "了解，先留意一下变化。"
	}
}

func draftNote(seg *segment, r *rng) string {
	switch seg.Kind {
	case kindLife:
		base := fmt.Sprintf("第%d周（%s），用户提到这周主要在%s，%s，没有特别的诉求。",
			seg.Week+1, seg.At.Format("2006-01-02"),
			r.pick([]string{"家里和公司两点一线", "忙工作、周末补觉", "处理家里的琐事", "调整作息", "在附近随便走走"}),
			r.pick([]string{"情绪比较平稳", "有点疲惫但还能应付", "状态比前几周好一些", "没怎么出门"}))
		return padNote(base, seg.TargetLen, r)
	default:
		base := fmt.Sprintf("第%d周（%s），用户对回复方式提了一个小要求：%s。这属于沟通偏好，不涉及健康事实。",
			seg.Week+1, seg.At.Format("2006-01-02"), styleGeneric[r.intn(len(styleGeneric))])
		return padNote(base, seg.TargetLen, r)
	}
}

func padNote(base string, target int, r *rng) string {
	out := base
	pool := []string{
		"当时只是顺带聊到，没有要求给建议。",
		"助手做了简短回应，没有展开。",
		"这条只作为对话记录保留，不构成任何健康结论。",
		"用户没有继续追问，话题就停在这里。",
		"语气比较轻松，看起来并不是求助。",
		"后续没有再提到同一件事。",
	}
	start := r.intn(len(pool))
	for utf8.RuneCountInString(out) < target {
		out += pool[start%len(pool)]
		start++
	}
	return out
}

func anchorNoteText(seg *segment, a *anchor, r *rng) string {
	var base string
	if a.Kind == kindLife {
		base = fmt.Sprintf("第%d周（%s），用户提到这周去了%s%s，%s。",
			seg.Week+1, seg.At.Format("2006-01-02"), a.Place, a.Action, a.Detail)
		base += fmt.Sprintf("%s，%s，%s。", r.pick(lifeWeather), r.pick(lifeCompany), r.pick(lifeFeeling))
	} else {
		base = fmt.Sprintf("第%d周（%s），用户明确提出沟通偏好：%s（用户原话大意：「%s」）。这条只约束回复风格，不涉及健康事实。",
			seg.Week+1, seg.At.Format("2006-01-02"), a.Request, a.Term)
	}
	return padNote(base, seg.TargetLen, r)
}

// assignAnchors gives 50 notes spread over the newest 30 note-eligible
// segments of each user. That range is inside the retained suffix of every
// measured cap, so every annotated note exists in the recall index.
func (c *corpus) assignAnchors() {
	type candidate struct {
		seg *segment
	}
	var candidates []candidate
	for _, u := range c.Users {
		var eligible []*segment
		for _, s := range u.Segments {
			if s.Eligible {
				eligible = append(eligible, s)
			}
		}
		if len(eligible) > 30 {
			eligible = eligible[len(eligible)-30:]
		}
		for _, s := range eligible {
			candidates = append(candidates, candidate{seg: s})
		}
	}
	r := newRNG(c.Seed ^ 0x5eed)
	for i := range anchors {
		if len(candidates) == 0 {
			return
		}
		pos := i * len(candidates) / len(anchors)
		seg := candidates[pos].seg
		seg.Anchor = &anchors[i]
		seg.NoteText = anchorNoteText(seg, &anchors[i], r)
	}
}

type retentionCap struct {
	Name   string
	Count  int
	Runes  int
	Source string
}

var retentionCaps = []retentionCap{
	{Name: "test-only", Count: 48, Runes: 16000, Source: "historical S2-A test-only tier (superseded by the approved policy)"},
	{Name: "candidate-a", Count: 96, Runes: 32000, Source: "approved default retention policy (product owner, 2026-11-05)"},
	{Name: "candidate-b", Count: 200, Runes: 64000, Source: "candidate cap for product review"},
}

type rolloutStat struct {
	CapName       string        `json:"cap"`
	CountCap      int           `json:"count_limit"`
	RuneCap       int           `json:"character_limit"`
	Users         int           `json:"users"`
	EligibleNotes int           `json:"eligible_notes"`
	Retained      int           `json:"retained_notes"`
	RetainedRunes int           `json:"retained_characters"`
	RolledOut     int           `json:"rolled_out_notes"`
	RolloutRatio  float64       `json:"rollout_ratio"`
	Binding       string        `json:"binding_constraint"`
	PerUser       []userRollout `json:"per_user"`
}

type userRollout struct {
	UserIndex  int      `json:"user_index"`
	Eligible   int      `json:"eligible_notes"`
	Retained   int      `json:"retained_notes"`
	Runes      int      `json:"retained_characters"`
	RolledOut  int      `json:"rolled_out_notes"`
	Binding    string   `json:"binding_constraint"`
	RetainedID []string `json:"-"`
}

// simulateRollout is a pure in-memory mirror of the Go rollover loop. It is
// used only to validate fixtures and to cross-check the PostgreSQL run.
func (c *corpus) simulateRollout(cap retentionCap) rolloutStat {
	stat := rolloutStat{CapName: cap.Name, CountCap: cap.Count, RuneCap: cap.Runes, Users: len(c.Users)}
	for _, u := range c.Users {
		var retained []*segment
		total := 0
		binding := "none"
		rolled := 0
		for _, s := range u.Segments {
			if !s.Eligible {
				continue
			}
			retained = append(retained, s)
			total += utf8.RuneCountInString(s.NoteText)
			for len(retained) > cap.Count || total > cap.Runes {
				if len(retained) == 0 {
					break
				}
				total -= utf8.RuneCountInString(retained[0].NoteText)
				retained = retained[1:]
				rolled++
				if len(retained)+1 > cap.Count {
					binding = "count"
				} else {
					binding = "characters"
				}
			}
		}
		ur := userRollout{UserIndex: u.Index, Eligible: len(retained) + rolled, Retained: len(retained), Runes: total, RolledOut: rolled, Binding: binding}
		for _, s := range retained {
			ur.RetainedID = append(ur.RetainedID, s.NoteID.String())
		}
		stat.PerUser = append(stat.PerUser, ur)
		stat.EligibleNotes += ur.Eligible
		stat.Retained += ur.Retained
		stat.RetainedRunes += ur.Runes
		stat.RolledOut += ur.RolledOut
	}
	if stat.EligibleNotes > 0 {
		stat.RolloutRatio = float64(stat.RolledOut) / float64(stat.EligibleNotes)
	}
	stat.Binding = bindingOf(stat.PerUser)
	return stat
}

func bindingOf(users []userRollout) string {
	count, chars := 0, 0
	for _, u := range users {
		switch u.Binding {
		case "count":
			count++
		case "characters":
			chars++
		}
	}
	switch {
	case count == 0 && chars == 0:
		return "none"
	case chars == 0:
		return "count"
	case count == 0:
		return "characters"
	default:
		return "mixed"
	}
}
