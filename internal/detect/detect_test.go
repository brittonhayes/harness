package detect

import (
	"strings"
	"testing"
)

const validRule = `
title: AWS Root Console Login
id: 8a7b6c5d-1234-4abc-9def-0123456789ab
status: experimental
description: Detects root account console logins.
logsource:
  product: aws
  service: cloudtrail
detection:
  selection:
    eventName: ConsoleLogin
    userIdentity.type: Root
  condition: selection
level: high
tags:
  - attack.initial_access
  - attack.t1078.004
`

func TestValidateValidRule(t *testing.T) {
	issues, err := ValidateBytes([]byte(validRule))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(issues) != 0 {
		t.Fatalf("expected no issues, got: %v", issues)
	}
}

func TestValidateMissingRequired(t *testing.T) {
	// Missing the required "detection" property.
	rule := `
title: Incomplete Rule
logsource:
  product: aws
`
	issues, err := ValidateBytes([]byte(rule))
	if err != nil {
		t.Fatal(err)
	}
	if len(issues) == 0 {
		t.Fatal("expected validation issues for missing detection, got none")
	}
	joined := joinIssues(issues)
	if !strings.Contains(strings.ToLower(joined), "detection") {
		t.Fatalf("expected an issue mentioning 'detection', got: %s", joined)
	}
}

func TestValidateBadLevelEnum(t *testing.T) {
	rule := strings.Replace(validRule, "level: high", "level: catastrophic", 1)
	issues, err := ValidateBytes([]byte(rule))
	if err != nil {
		t.Fatal(err)
	}
	if len(issues) == 0 {
		t.Fatal("expected an issue for invalid level enum, got none")
	}
}

func TestValidateMultiDoc(t *testing.T) {
	// Two valid docs separated by ---; second is missing detection.
	multi := validRule + "\n---\n" + `
title: Second Rule
logsource:
  product: aws
`
	issues, err := ValidateBytes([]byte(multi))
	if err != nil {
		t.Fatal(err)
	}
	if len(issues) == 0 {
		t.Fatal("expected issues from the second document")
	}
	for _, is := range issues {
		if is.Doc != 2 {
			t.Fatalf("expected all issues from doc 2, got doc %d (%s)", is.Doc, is.Msg)
		}
	}
}

func joinIssues(issues []Issue) string {
	parts := make([]string, len(issues))
	for i, is := range issues {
		parts[i] = is.String()
	}
	return strings.Join(parts, "; ")
}
