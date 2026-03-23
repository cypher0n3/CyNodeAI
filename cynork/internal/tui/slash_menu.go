package tui

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// slashMenuMaxVisible is the max number of catalog rows shown before scrolling.
const slashMenuMaxVisible = 8

var (
	slashMenuSepStyle     = lipgloss.NewStyle().Foreground(lipgloss.Color("240"))
	slashMenuSelStyle     = lipgloss.NewStyle().Foreground(lipgloss.Color("255")).Bold(true)
	slashMenuDimStyle     = lipgloss.NewStyle().Foreground(lipgloss.Color("245"))
	slashMenuDescStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("241"))
	slashMenuFooterStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("240"))
	slashMenuNoMatchStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("244"))
)

// activeComposerLine returns the line the user is editing (last line in multiline input).
func activeComposerLine(input string) string {
	if input == "" {
		return ""
	}
	lines := strings.Split(input, "\n")
	return lines[len(lines)-1]
}

// slashMenuEligible is true when the first non-whitespace character on the active line is '/'.
func slashMenuEligible(line string) bool {
	for _, r := range line {
		if r == '/' {
			return true
		}
		if r != ' ' && r != '\t' {
			return false
		}
	}
	return false
}

// catalogHasExact reports whether the catalog has an entry whose name equals line (case-insensitive).
func catalogHasExact(line string) bool {
	line = strings.TrimSpace(line)
	for _, e := range slashHelpCatalog {
		if strings.EqualFold(e.name, line) {
			return true
		}
	}
	return false
}

// filteredSlashCommands returns catalog entries whose command name prefix-matches the active line.
// After a space following a command that exists as an exact catalog entry (e.g. "/auth "), only
// subcommands (names starting with "/auth ") are listed; without a trailing space, multi-segment
// names are hidden so Tab narrows to "/auth" first, then subcommands after a space.
func (m *Model) filteredSlashCommands() []struct{ name, desc string } {
	lineRaw := activeComposerLine(m.Input)
	if !slashMenuEligible(lineRaw) {
		return nil
	}
	line := strings.TrimLeft(lineRaw, " \t")
	if !strings.HasPrefix(line, "/") {
		return nil
	}
	prefixLower := strings.ToLower(line)
	trimForExact := strings.TrimSpace(line)
	exactExists := catalogHasExact(trimForExact)
	hasTrailingSpace := strings.HasSuffix(line, " ") || strings.HasSuffix(line, "\t")

	var out []struct{ name, desc string }
	for _, e := range slashHelpCatalog {
		el := strings.ToLower(e.name)
		if !strings.HasPrefix(el, prefixLower) {
			continue
		}
		if exactExists && !hasTrailingSpace && prefixLower != "" && strings.HasPrefix(el, prefixLower+" ") {
			continue
		}
		out = append(out, e)
	}
	return out
}

// slashMenuVisible is true when the popup should reserve layout space.
func (m *Model) slashMenuVisible() bool {
	if m.Loading || m.ShowLoginForm {
		return false
	}
	line := activeComposerLine(m.Input)
	return slashMenuEligible(line) && strings.HasPrefix(strings.TrimSpace(line), "/")
}

func (m *Model) clampSlashMenuSelection() {
	filtered := m.filteredSlashCommands()
	if len(filtered) == 0 {
		m.slashMenuSel = 0
		m.slashMenuScroll = 0
		return
	}
	if m.slashMenuSel >= len(filtered) {
		m.slashMenuSel = len(filtered) - 1
	}
	if m.slashMenuSel < 0 {
		m.slashMenuSel = 0
	}
	m.ensureSlashMenuScrollVisible()
}

func (m *Model) ensureSlashMenuScrollVisible() {
	filtered := m.filteredSlashCommands()
	if len(filtered) == 0 {
		return
	}
	if m.slashMenuSel < m.slashMenuScroll {
		m.slashMenuScroll = m.slashMenuSel
	}
	if m.slashMenuSel >= m.slashMenuScroll+slashMenuMaxVisible {
		m.slashMenuScroll = m.slashMenuSel - slashMenuMaxVisible + 1
	}
	if m.slashMenuScroll < 0 {
		m.slashMenuScroll = 0
	}
	maxScroll := len(filtered) - slashMenuMaxVisible
	if maxScroll < 0 {
		maxScroll = 0
	}
	if m.slashMenuScroll > maxScroll {
		m.slashMenuScroll = maxScroll
	}
}

func (m *Model) navSlashMenu(up bool) {
	filtered := m.filteredSlashCommands()
	if len(filtered) == 0 {
		return
	}
	if up {
		m.slashMenuSel--
		if m.slashMenuSel < 0 {
			m.slashMenuSel = len(filtered) - 1
		}
	} else {
		m.slashMenuSel++
		if m.slashMenuSel >= len(filtered) {
			m.slashMenuSel = 0
		}
	}
	m.ensureSlashMenuScrollVisible()
}

func (m *Model) replaceActiveComposerLine(newLastLine string) {
	lines := strings.Split(m.Input, "\n")
	if len(lines) == 0 {
		m.Input = newLastLine
		m.syncInputCursorEnd()
		return
	}
	lines[len(lines)-1] = newLastLine
	m.Input = strings.Join(lines, "\n")
	m.syncInputCursorEnd()
}

func (m *Model) applySlashCompletion() {
	filtered := m.filteredSlashCommands()
	if len(filtered) == 0 {
		return
	}
	sel := m.slashMenuSel
	if sel < 0 || sel >= len(filtered) {
		sel = 0
	}
	chosen := filtered[sel].name
	m.replaceActiveComposerLine(chosen + " ")
}

func slashMenuColumnWidths(termW int) (nameW, descW int) {
	w := termW
	if w < 10 {
		w = 10
	}
	nameW = min(36, w/3)
	if nameW < 12 {
		nameW = 12
	}
	descW = w - nameW - 4
	if descW < 8 {
		descW = 8
	}
	return nameW, descW
}

func formatSlashMenuRow(name, desc string, isSel bool, nameW, descW int) string {
	if lipgloss.Width(name) > nameW {
		name = truncateRunes(name, nameW-1) + "…"
	}
	if lipgloss.Width(desc) > descW {
		desc = truncateRunes(desc, descW-1) + "…"
	}
	prefix := "  "
	if isSel {
		prefix = "→ "
	}
	nameSt := slashMenuDimStyle.Width(nameW).Render(name)
	if isSel {
		nameSt = slashMenuSelStyle.Width(nameW).Render(name)
	}
	descSt := slashMenuDescStyle.Width(descW).Render(desc)
	return prefix + nameSt + "  " + descSt + "\n"
}

// renderSlashMenuBlock renders the hint list below the composer (empty string if hidden).
func (m *Model) renderSlashMenuBlock() string {
	if !m.slashMenuVisible() {
		return ""
	}
	filtered := m.filteredSlashCommands()
	w := m.Width
	if w < 10 {
		w = 10
	}
	sepW := min(w, 80)
	sep := slashMenuSepStyle.Width(w).Render(strings.Repeat("─", sepW))
	var b strings.Builder
	b.WriteString(sep)
	b.WriteString("\n")

	if len(filtered) == 0 {
		b.WriteString(slashMenuNoMatchStyle.Render("No matching slash commands"))
		return b.String()
	}

	start := m.slashMenuScroll
	end := start + slashMenuMaxVisible
	if end > len(filtered) {
		end = len(filtered)
	}
	nameW, descW := slashMenuColumnWidths(m.Width)

	for i := start; i < end; i++ {
		e := filtered[i]
		b.WriteString(formatSlashMenuRow(e.name, e.desc, i == m.slashMenuSel, nameW, descW))
	}
	if len(filtered) > slashMenuMaxVisible {
		b.WriteString(slashMenuFooterStyle.Render("↓ more below"))
		b.WriteString("\n")
	}
	return strings.TrimRight(b.String(), "\n")
}

func truncateRunes(s string, maxCells int) string {
	if maxCells < 1 {
		return ""
	}
	w := 0
	for i, r := range s {
		rw := lipgloss.Width(string(r))
		if w+rw > maxCells {
			return s[:i]
		}
		w += rw
	}
	return s
}
