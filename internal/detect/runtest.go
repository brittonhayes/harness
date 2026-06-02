package detect

import (
	"bytes"
	"fmt"
	"io"
	"os"

	"gopkg.in/yaml.v3"
)

// Inline test cases let a detection ship with sample events that the engine
// runs, so a rule's logic is verifiable — not just schema-valid. Cases live in
// a custom top-level `tests:` field, which the Sigma schema permits:
//
//	tests:
//	  - name: root console login fires
//	    event: { eventName: ConsoleLogin, userIdentity.type: Root }
//	    match: true

// TestOutcome is the result of running one inline test case.
type TestOutcome struct {
	Name string
	Want bool   // expected match
	Got  bool   // actual match
	Err  string // evaluation error, if any
}

// Passed reports whether the case behaved as expected (and didn't error).
func (o TestOutcome) Passed() bool { return o.Err == "" && o.Got == o.Want }

// RuleTestResult collects the outcomes for one rule document.
type RuleTestResult struct {
	Path  string
	Doc   int // 1-based document index within the file
	Title string
	Cases []TestOutcome
	Err   string // document-level error (no tests, unparseable, etc.)
}

// Passed reports whether the rule has at least one case and all cases passed.
func (r RuleTestResult) Passed() bool {
	if r.Err != "" || len(r.Cases) == 0 {
		return false
	}
	for _, c := range r.Cases {
		if !c.Passed() {
			return false
		}
	}
	return true
}

// inlineTest is the decoded shape of one `tests:` entry.
type inlineTest struct {
	Name  string         `yaml:"name"`
	Event map[string]any `yaml:"event"`
	Match bool           `yaml:"match"`
}

// TestBytes runs the inline tests of every document in a Sigma file body. A
// document without a `tests:` field yields a result with Err set, so callers
// can flag rules that ship without tests.
func TestBytes(data []byte) ([]RuleTestResult, error) {
	dec := yaml.NewDecoder(bytes.NewReader(data))
	var out []RuleTestResult
	doc := 0
	for {
		var raw any
		err := dec.Decode(&raw)
		if err == io.EOF {
			break
		}
		doc++
		if err != nil {
			out = append(out, RuleTestResult{Doc: doc, Err: "YAML parse error: " + err.Error()})
			break
		}
		if raw == nil {
			continue
		}
		rule, ok := normalize(raw).(map[string]any)
		if !ok {
			out = append(out, RuleTestResult{Doc: doc, Err: "rule is not a mapping"})
			continue
		}
		out = append(out, testRule(rule, doc))
	}
	return out, nil
}

func testRule(rule map[string]any, doc int) RuleTestResult {
	res := RuleTestResult{Doc: doc}
	if t, ok := rule["title"].(string); ok {
		res.Title = t
	}
	cases, err := decodeTests(rule["tests"])
	if err != nil {
		res.Err = err.Error()
		return res
	}
	if len(cases) == 0 {
		res.Err = "no inline tests (add a 'tests:' field)"
		return res
	}
	for _, c := range cases {
		out := TestOutcome{Name: c.Name, Want: c.Match}
		got, err := EvaluateRule(rule, c.Event)
		if err != nil {
			out.Err = err.Error()
		} else {
			out.Got = got
		}
		res.Cases = append(res.Cases, out)
	}
	return res
}

// decodeTests re-decodes the raw `tests` value into typed cases.
func decodeTests(raw any) ([]inlineTest, error) {
	if raw == nil {
		return nil, nil
	}
	b, err := yaml.Marshal(raw)
	if err != nil {
		return nil, fmt.Errorf("cannot read tests field: %w", err)
	}
	var cases []inlineTest
	if err := yaml.Unmarshal(b, &cases); err != nil {
		return nil, fmt.Errorf("invalid tests field: %w", err)
	}
	for i, c := range cases {
		if c.Event == nil {
			return nil, fmt.Errorf("test %d (%q) has no event", i+1, c.Name)
		}
		// Normalize nested event maps to JSON-compatible types.
		cases[i].Event, _ = normalize(c.Event).(map[string]any)
	}
	return cases, nil
}

// TestFile runs the inline tests in a single rule file.
func TestFile(path string) []RuleTestResult {
	data, err := os.ReadFile(path)
	if err != nil {
		return []RuleTestResult{{Path: path, Err: "cannot read file: " + err.Error()}}
	}
	results, err := TestBytes(data)
	if err != nil {
		return []RuleTestResult{{Path: path, Err: err.Error()}}
	}
	for i := range results {
		results[i].Path = path
	}
	return results
}
