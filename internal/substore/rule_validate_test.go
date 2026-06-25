package substore

import "testing"

func TestValidateRuleContent(t *testing.T) {
	content := `# 这是注释,跳过
DOMAIN-SUFFIX,anthropic.com
RULE-SET,openai,🤖 AI 服务
MATCH
MATCH,🚀 节点选择
GEOIP,CN,DIRECT,no-resolve
AND,((DOMAIN,a.com),(NETWORK,tcp)),🚀 节点选择
DOMAINSUFFIX,b.com,DIRECT`
	issues := ValidateRuleContent(content)
	// Bad lines: DOMAIN-SUFFIX,anthropic.com (line 2), MATCH (line 4),
	// DOMAINSUFFIX,... typo (line 8). The rest are valid.
	if len(issues) != 3 {
		t.Fatalf("want 3 issues, got %d: %+v", len(issues), issues)
	}
	byLine := map[int]string{}
	for _, is := range issues {
		byLine[is.Line] = is.Suggestion
	}
	if got := byLine[2]; got != "DOMAIN-SUFFIX,anthropic.com,<策略组>" {
		t.Errorf("line 2 suggestion = %q", got)
	}
	if _, ok := byLine[4]; !ok {
		t.Errorf("MATCH without policy should be flagged")
	}
	if got := byLine[8]; got != "DOMAIN-SUFFIX,b.com,DIRECT" {
		t.Errorf("typo line 8 suggestion = %q, want did-you-mean fix", got)
	}
}

func TestRenderableRuleLine_DropsBroken(t *testing.T) {
	// parseRuleLines must keep valid lines, drop broken ones + comments.
	content := `# note
DOMAIN-SUFFIX,anthropic.com
RULE-SET,openai,🤖 AI 服务
MATCH,🚀 节点选择`
	got := parseRuleLines(content)
	want := []string{"RULE-SET,openai,🤖 AI 服务", "MATCH,🚀 节点选择"}
	if len(got) != len(want) {
		t.Fatalf("got %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("line %d = %q, want %q", i, got[i], want[i])
		}
	}
}

func TestValidateRuleContent_AllValid(t *testing.T) {
	content := "RULE-SET,cn-domain,DIRECT\nGEOIP,CN,DIRECT,no-resolve\nMATCH,🚀 节点选择"
	if issues := ValidateRuleContent(content); len(issues) != 0 {
		t.Fatalf("expected no issues, got %+v", issues)
	}
}
