package tools

import (
	"fmt"
	"strings"

	"github.com/brittonhayes/vala/internal/brain"
	"github.com/brittonhayes/vala/internal/tool"
)

func textWithCard(content string, card tool.Card) tool.Result {
	res := tool.Text(content)
	res.Card = &card
	return res
}

func fields(items ...tool.Field) []tool.Field {
	out := make([]tool.Field, 0, len(items))
	for _, item := range items {
		if strings.TrimSpace(item.Value) == "" {
			continue
		}
		out = append(out, item)
	}
	return out
}

func field(label, value string) tool.Field {
	return tool.Field{Label: label, Value: value}
}

func join(values []string) string {
	var out []string
	for _, v := range values {
		if s := strings.TrimSpace(v); s != "" {
			out = append(out, s)
		}
	}
	return strings.Join(out, ", ")
}

func defaultPriority(priority string) string {
	if strings.TrimSpace(priority) != "" {
		return priority
	}
	return "medium"
}

func visibilityGapSuggestion(sources []string, gap string) tool.Suggestion {
	dataSource := join(sources)
	if strings.TrimSpace(gap) == "" {
		gap = "telemetry not validated for " + dataSource
	}
	return tool.Suggestion{
		Title:      "Close the visibility gap",
		Trigger:    "Visibility gap: " + gap,
		Hypothesis: "Required telemetry can be made available and validated for future hunts.",
		Behavior:   "forensic readiness for " + gap,
		DataSource: dataSource,
		Priority:   "high",
	}
}

func huntFollowUpSuggestion(title, trigger, hypothesis, priority, mitre string) tool.Suggestion {
	return tool.Suggestion{
		Title:      title,
		Trigger:    trigger,
		Hypothesis: hypothesis,
		Priority:   defaultPriority(priority),
		MITRE:      mitre,
	}
}

func coverageSuggestion(technique, status string) tool.Suggestion {
	return tool.Suggestion{
		Title:      "Improve thin coverage",
		Trigger:    fmt.Sprintf("Coverage for %s is %s", technique, status),
		Hypothesis: fmt.Sprintf("%s has huntable behavior that can raise coverage quality.", technique),
		Behavior:   "identify a reliable detection or repeatable hunt for " + technique,
		Priority:   "medium",
		MITRE:      technique,
	}
}

func storeHuntSuggestions(outcome, tier string, nextSteps []string, question string) []tool.Suggestion {
	var suggestions []tool.Suggestion
	if outcome == brain.HuntInconclusive {
		suggestions = append(suggestions, huntFollowUpSuggestion(
			"Resolve inconclusive hunt",
			"Inconclusive hunt: "+question,
			"Additional evidence can confirm or refute the original hypothesis.",
			"medium",
			"",
		))
	}
	switch tier {
	case brain.TierRecurring:
		suggestions = append(suggestions, huntFollowUpSuggestion(
			"Schedule recurring hunt",
			"Tier 3 decision: recurring hunt needed",
			"The behavior is worth re-running on a cadence until it can be automated or retired.",
			"medium",
			"",
		))
	case brain.TierPlaybook:
		suggestions = append(suggestions, huntFollowUpSuggestion(
			"Turn method into a playbook",
			"Tier 4 decision: playbook needed",
			"The investigation method can be documented so future hunts repeat it consistently.",
			"medium",
			"",
		))
	case brain.TierNoDetection:
		suggestions = append(suggestions, huntFollowUpSuggestion(
			"Revisit no-detection decision",
			"Tier 5 decision: no detection documented",
			"A future trigger or visibility improvement could justify reopening this behavior.",
			"low",
			"",
		))
	}
	for _, step := range nextSteps {
		step = strings.TrimSpace(step)
		if step == "" {
			continue
		}
		suggestions = append(suggestions, huntFollowUpSuggestion(
			step,
			"Next step from closed hunt: "+step,
			"Follow-up work can test or retire this next step.",
			"medium",
			"",
		))
	}
	return suggestions
}
