package detect

import "testing"

func event(pairs ...any) map[string]any {
	m := map[string]any{}
	for i := 0; i+1 < len(pairs); i += 2 {
		m[pairs[i].(string)] = pairs[i+1]
	}
	return m
}

func TestMatchField(t *testing.T) {
	tests := []struct {
		name     string
		key      string
		expected any
		event    map[string]any
		want     bool
	}{
		{"exact string", "eventName", "ConsoleLogin", event("eventName", "ConsoleLogin"), true},
		{"case insensitive", "eventName", "consolelogin", event("eventName", "ConsoleLogin"), true},
		{"mismatch", "eventName", "GetObject", event("eventName", "ConsoleLogin"), false},
		{"list OR match", "eventName", []any{"GetObject", "ConsoleLogin"}, event("eventName", "ConsoleLogin"), true},
		{"dotted nested", "userIdentity.type", "Root", event("userIdentity", map[string]any{"type": "Root"}), true},
		{"dotted flat", "userIdentity.type", "Root", event("userIdentity.type", "Root"), true},
		{"null absent", "errorCode", nil, event("eventName", "X"), true},
		{"null present fails", "errorCode", nil, event("errorCode", "AccessDenied"), false},
		{"contains", "requestParameters|contains", "secret", event("requestParameters", "my-secret-key"), true},
		{"startswith", "eventName|startswith", "Delete", event("eventName", "DeleteTrail"), true},
		{"endswith", "eventName|endswith", "Trail", event("eventName", "DeleteTrail"), true},
		{"wildcard", "userAgent", "*Boto3*", event("userAgent", "aws-cli Boto3/1.0"), true},
		{"regex", "eventName|re", "^Delete.*", event("eventName", "DeleteTrail"), true},
		{"cidr match", "sourceIPAddress|cidr", "10.0.0.0/8", event("sourceIPAddress", "10.1.2.3"), true},
		{"cidr miss", "sourceIPAddress|cidr", "10.0.0.0/8", event("sourceIPAddress", "192.168.1.1"), false},
		{"numeric gt", "count|gt", 5, event("count", 10), true},
		{"numeric gt false", "count|gt", 5, event("count", 2), false},
		{"all modifier", "tags|all", []any{"a", "b"}, event("tags", []any{"a", "b", "c"}), true},
		{"all modifier missing", "tags|all", []any{"a", "z"}, event("tags", []any{"a", "b"}), false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := matchField(tt.event, tt.key, tt.expected)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tt.want {
				t.Fatalf("matchField(%q,%v) = %v, want %v", tt.key, tt.expected, got, tt.want)
			}
		})
	}
}

func TestUnsupportedModifier(t *testing.T) {
	_, err := matchField(event("data", "x"), "data|base64", "x")
	if err == nil {
		t.Fatal("expected error for unsupported modifier, got nil")
	}
}

func TestEvaluateRuleCondition(t *testing.T) {
	rule := map[string]any{
		"detection": map[string]any{
			"selection": map[string]any{"userIdentity.type": "Root"},
			"filter_service_event": map[string]any{
				"eventType": "AwsServiceEvent",
			},
			"condition": "selection and not filter_service_event",
		},
	}
	tests := []struct {
		name  string
		event map[string]any
		want  bool
	}{
		{"root interactive fires", event("userIdentity.type", "Root"), true},
		{"non-root no fire", event("userIdentity.type", "IAMUser"), false},
		{"service event filtered", event("userIdentity.type", "Root", "eventType", "AwsServiceEvent"), false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := EvaluateRule(rule, tt.event)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tt.want {
				t.Fatalf("EvaluateRule = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestConditionQuantifiers(t *testing.T) {
	results := map[string]bool{"selection_a": true, "selection_b": false, "selection_c": true}
	names := []string{"selection_a", "selection_b", "selection_c"}
	cases := []struct {
		cond string
		want bool
	}{
		{"1 of selection*", true},
		{"all of selection*", false},
		{"2 of selection*", true},
		{"selection_a and selection_c", true},
		{"selection_a and selection_b", false},
		{"selection_a or selection_b", true},
		{"not selection_b", true},
		{"(selection_a or selection_b) and selection_c", true},
		{"all of them", false},
		{"1 of them", true},
	}
	for _, c := range cases {
		got, err := evalCondition(c.cond, names, results)
		if err != nil {
			t.Fatalf("%q: unexpected error: %v", c.cond, err)
		}
		if got != c.want {
			t.Fatalf("evalCondition(%q) = %v, want %v", c.cond, got, c.want)
		}
	}
}

func TestConditionAggregationUnsupported(t *testing.T) {
	_, err := evalCondition("selection | count() > 5", []string{"selection"}, map[string]bool{"selection": true})
	if err == nil {
		t.Fatal("expected error for aggregation condition")
	}
}

func TestTestBytes(t *testing.T) {
	rule := []byte(`
title: Root Usage
logsource:
  product: aws
  service: cloudtrail
detection:
  selection:
    userIdentity.type: Root
  filter_service_event:
    eventType: AwsServiceEvent
  condition: selection and not filter_service_event
tests:
  - name: root fires
    event:
      userIdentity.type: Root
    match: true
  - name: service event filtered
    event:
      userIdentity.type: Root
      eventType: AwsServiceEvent
    match: false
`)
	results, err := TestBytes(rule)
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 rule result, got %d", len(results))
	}
	if !results[0].Passed() {
		t.Fatalf("expected all cases to pass: %+v", results[0])
	}
}
