package collector

import "testing"

func testEntity() Entity {
	return Entity{
		Name:             "田曦薇",
		Aliases:          []string{"小田"},
		VerifiedAccounts: []string{"田曦薇工作室"},
		ExcludedTerms: []string{
			"田姓氏", "田字解释", "汉字田", "田字组词",
			"田的部首", "田的拼音", "田的笔画", "田字成语", "百家姓",
		},
		WorkTitles:     []string{"逐玉", "半熟男女"},
		CharacterNames: []string{"李薇"},
		EventTitles:    []string{"品牌发布会"},
	}
}

func TestEvaluateFiltersDictionaryContent(t *testing.T) {
	cases := []struct {
		name    string
		title   string
		summary string
	}{
		{"surname encyclopedia", "田姓氏的来源和分布", "田姓，中华姓氏之一，分布广泛。"},
		{"character explanation", "田字解释_田的意思_田的拼音_田的部首", "田字的基本释义与读音。"},
		{"stroke order", "汉字田的笔画笔顺", "田字的笔画数、笔顺和组词示例。"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := Evaluate(testEntity(), EvaluateInput{
				Title: tc.title, Summary: tc.summary, MetadataStatus: "ok",
			})
			if got.Label != LabelFiltered {
				t.Fatalf("label = %q, want filtered", got.Label)
			}
			if got.FilterReason != FilterDictionaryContent {
				t.Fatalf("filter reason = %q, want %q", got.FilterReason, FilterDictionaryContent)
			}
			if got.Score != 0 {
				t.Fatalf("score = %d, want 0", got.Score)
			}
		})
	}
}

func TestEvaluateKeepsRealArtistContent(t *testing.T) {
	got := Evaluate(testEntity(), EvaluateInput{
		Title:          "田曦薇新剧《逐玉》开机，搭档张凌赫",
		Summary:        "演员田曦薇出席开机仪式，新剧逐玉正式启动拍摄。",
		Author:         "某娱乐媒体",
		MetadataStatus: "ok",
	})
	if got.Label != LabelHigh {
		t.Fatalf("label = %q, want high (score %d): %s", got.Label, got.Score, got.Reason)
	}
	if got.FilterReason != "" {
		t.Fatalf("filter reason = %q, want empty", got.FilterReason)
	}
	if got.Score < thresholdHigh {
		t.Fatalf("score = %d, want >= %d", got.Score, thresholdHigh)
	}
}

func TestEvaluateAliasInTitleCounts(t *testing.T) {
	got := Evaluate(testEntity(), EvaluateInput{
		Title:          "小田现身机场，状态轻松",
		Summary:        "粉丝拍到小田抵达机场。",
		MetadataStatus: "ok",
	})
	if got.Label != LabelCandidate {
		t.Fatalf("label = %q, want candidate (score %d)", got.Label, got.Score)
	}
}

func TestEvaluateWorkOnlyIsFiltered(t *testing.T) {
	got := Evaluate(testEntity(), EvaluateInput{
		Title:          "《逐玉》发布定档预告",
		Summary:        "古装爱情剧逐玉今日发布定档预告片。",
		MetadataStatus: "ok",
	})
	if got.Label != LabelFiltered {
		t.Fatalf("label = %q, want filtered (score %d)", got.Label, got.Score)
	}
	if got.FilterReason != FilterInsufficientCtx {
		t.Fatalf("filter reason = %q, want %q", got.FilterReason, FilterInsufficientCtx)
	}
	if got.Score != ScoreRelated {
		t.Fatalf("score = %d, want %d", got.Score, ScoreRelated)
	}
}

func TestEvaluateVerifiedAccountWithRelatedWorkIsCandidate(t *testing.T) {
	got := Evaluate(testEntity(), EvaluateInput{
		Title:          "《逐玉》今日开机",
		Summary:        "剧组已正式开机。",
		Author:         "田曦薇工作室",
		MetadataStatus: "ok",
	})
	if got.Label != LabelCandidate {
		t.Fatalf("label = %q, want candidate (score %d): %s", got.Label, got.Score, got.Reason)
	}
	if got.Score != ScoreVerifiedAccount+ScoreRelated {
		t.Fatalf("score = %d, want %d", got.Score, ScoreVerifiedAccount+ScoreRelated)
	}
}

func TestEvaluateSurnameOnlyIsAmbiguous(t *testing.T) {
	got := Evaluate(testEntity(), EvaluateInput{
		Title:          "田氏家族的历史与分布",
		Summary:        "田氏起源于春秋时期，后逐渐分布各地。",
		MetadataStatus: "ok",
	})
	if got.Label != LabelFiltered {
		t.Fatalf("label = %q, want filtered", got.Label)
	}
	if got.FilterReason != FilterAmbiguousEntity {
		t.Fatalf("filter reason = %q, want %q", got.FilterReason, FilterAmbiguousEntity)
	}
}

func TestEvaluateLowContextPenalty(t *testing.T) {
	got := Evaluate(testEntity(), EvaluateInput{
		Title:          "一条没有人物上下文的动态",
		Summary:        "内容很短。",
		Author:         "田曦薇工作室",
		MetadataStatus: "failed",
	})
	if got.Label != LabelFiltered {
		t.Fatalf("label = %q, want filtered (score %d)", got.Label, got.Score)
	}
	if got.FilterReason != FilterInsufficientCtx {
		t.Fatalf("filter reason = %q, want %q", got.FilterReason, FilterInsufficientCtx)
	}
}

func TestValidateSearchQuery(t *testing.T) {
	names := []string{"田曦薇", "小田"}
	valid := []string{"田曦薇", "小田 近况", "田曦薇 作品 采访"}
	for _, query := range valid {
		if err := ValidateSearchQuery(query, names); err != nil {
			t.Fatalf("query %q rejected: %v", query, err)
		}
	}
	invalid := []string{"", "田", "演员", "女明星", "演员 采访", "新剧 动态"}
	for _, query := range invalid {
		if err := ValidateSearchQuery(query, names); err == nil {
			t.Fatalf("query %q accepted, want rejection", query)
		}
	}
}

func TestValidateSearchQueryRequiresConfiguredName(t *testing.T) {
	if err := ValidateSearchQuery("田曦薇", nil); err == nil {
		t.Fatal("expected error when no name is configured")
	}
}
