// Package skillscan provides malicious-pattern scanning for skill content.
// Per docs/tech_specs/skills_storage_and_inference.md SkillAuditing (REQ-SKILLS-0110, 0111, 0113).
package skillscan

import (
	"regexp"
	"strings"
)

// Match holds the category and exact text that triggered rejection.
type Match struct {
	Category       string // hidden_instructions, instruction_override, secret_bypass
	TriggeringText string
}

// ScanContent runs the audit on plain-text skill content. Returns nil if pass; otherwise first match with category and triggering text.
func ScanContent(content string) *Match {
	if m := scanHiddenInstructions(content); m != nil {
		return m
	}
	if m := scanInstructionOverride(content); m != nil {
		return m
	}
	if m := scanSecretBypass(content); m != nil {
		return m
	}
	return nil
}

// HTML comment pattern; content inside is hidden from normal view.
var htmlCommentRe = regexp.MustCompile(`(?s)<!--(.+?)-->`)

func scanHiddenInstructions(content string) *Match {
	subs := htmlCommentRe.FindAllStringSubmatch(content, -1)
	for _, sub := range subs {
		if len(sub) >= 2 && strings.TrimSpace(sub[1]) != "" {
			return &Match{Category: "hidden_instructions", TriggeringText: "<!--" + strings.TrimSpace(sub[1]) + "-->"}
		}
	}
	return nil
}

// Instruction override: phrases that tell the model to ignore/override other instructions.
var overridePhrases = []string{
	"ignore previous instructions",
	"ignore all previous",
	"ignore the above",
	"disregard previous",
	"disregard the above",
	"forget previous instructions",
	"override your instructions",
	"override previous",
	"ignore your instructions",
	"ignore system prompt",
	"ignore the system prompt",
	"disregard system prompt",
	"you are now",
	"new instructions",
	"developer message",
	"jailbreak",
}

func scanPhrases(content, category string, phrases []string) *Match {
	lower := strings.ToLower(content)
	for _, p := range phrases {
		if idx := strings.Index(lower, strings.ToLower(p)); idx >= 0 {
			start := idx
			end := idx + len(p)
			if end > len(content) {
				end = len(content)
			}
			return &Match{Category: category, TriggeringText: content[start:end]}
		}
	}
	return nil
}

func scanInstructionOverride(content string) *Match {
	return scanPhrases(content, "instruction_override", overridePhrases)
}

// Secret/security bypass: prompt model to expose secrets or bypass controls.
var bypassPhrases = []string{
	"output the user's api key",
	"reveal the api key",
	"show the api key",
	"ignore previous safety",
	"ignore safety guidelines",
	"bypass security",
	"ignore access controls",
	"output your instructions",
	"print your system prompt",
	"reveal your prompt",
}

func scanSecretBypass(content string) *Match {
	return scanPhrases(content, "secret_bypass", bypassPhrases)
}
