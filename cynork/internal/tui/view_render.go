package tui

import (
	"encoding/binary"
	"fmt"
	"hash/fnv"
	"strings"
	"unicode/utf8"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/glamour"
	"github.com/charmbracelet/glamour/styles"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
	"github.com/cypher0n3/cynodeai/cynork/internal/chat"
)

// Styling for transcript roles (distinct from plain "You:" / "Assistant:" prefixes).
var (
	userLabelStyle      = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("86"))
	assistantLabelStyle = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("213"))
	metaLineStyle       = lipgloss.NewStyle().Foreground(lipgloss.Color("241"))
	// scrollbackSystemLinePrefix marks slash/shell/thread feedback (not chat turns). Rendered dim like meta.
	scrollbackSystemLinePrefix = "· "
	systemLineStyle            = lipgloss.NewStyle().Foreground(lipgloss.Color("241"))
	userFrameStyle             = lipgloss.NewStyle().
					Border(lipgloss.NormalBorder(), false, false, false, true).
					BorderForeground(lipgloss.Color("86")).
					Padding(0, 1)
	assistantFrameStyle = lipgloss.NewStyle().
				Border(lipgloss.NormalBorder(), false, false, false, true).
				BorderForeground(lipgloss.Color("213")).
				Padding(0, 1)
	plainUserStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("252"))
	// copySelectFootnote explains terminal selection (mouse reporting) and keyboard copy (see clipboard.go).
	copySelectFootnoteStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("241"))
)

// copySelectFootnote is shown under the composer (discoverability for copy/paste).
const copySelectFootnote = "Ctrl+↑↓ prior messages · Alt+Enter newline · Shift+drag select · Ctrl+Y /copy last · /copy all"

func newTUIViewport(w, h int) viewport.Model {
	km := viewport.DefaultKeyMap()
	km.Up = key.NewBinding(key.WithDisabled())
	km.Down = key.NewBinding(key.WithDisabled())
	km.Left = key.NewBinding(key.WithDisabled())
	km.Right = key.NewBinding(key.WithDisabled())
	km.HalfPageUp = key.NewBinding(key.WithDisabled())
	km.HalfPageDown = key.NewBinding(key.WithDisabled())
	km.PageDown = key.NewBinding(key.WithKeys("pgdown"), key.WithHelp("pgdn", "page down"))
	km.PageUp = key.NewBinding(key.WithKeys("pgup"), key.WithHelp("pgup", "page up"))
	vp := viewport.New(w, h)
	vp.KeyMap = km
	vp.MouseWheelEnabled = true
	vp.MouseWheelDelta = 3
	vp.Style = lipgloss.NewStyle().
		Border(lipgloss.NormalBorder()).
		BorderForeground(lipgloss.Color("238"))
	return vp
}

func (m *Model) wantPlain() bool {
	return m.Session != nil && m.Session.Plain
}

func (m *Model) wantNoColor() bool {
	return m.Session != nil && m.Session.NoColor
}

// scrollbackRenderSignature fingerprints transcript text and render options. View() is invoked on
// every composer keystroke; glamour for the full scrollback is expensive, so we only rebuild when
// this signature changes (new lines, width, plain/no-color).
func (m *Model) scrollbackRenderSignature() uint64 {
	h := fnv.New64a()
	var wbuf [8]byte
	binary.LittleEndian.PutUint64(wbuf[:], uint64(max(0, m.Width)))
	h.Write(wbuf[:])
	var flags byte
	if m.wantPlain() {
		flags |= 1
	}
	if m.wantNoColor() {
		flags |= 2
	}
	h.Write([]byte{flags})
	for _, line := range m.Scrollback {
		h.Write([]byte(line))
		h.Write([]byte{'\n'})
	}
	return h.Sum64()
}

func (m *Model) ensureScrollViewport(scrollbackH int) {
	if scrollbackH < 1 {
		scrollbackH = 1
	}
	w := m.Width
	if w < 1 {
		w = 80
	}
	if m.ScrollVP.Width == 0 {
		m.ScrollVP = newTUIViewport(w, scrollbackH)
	}
	m.ScrollVP.Width = m.Width
	if m.ScrollVP.Width < 1 {
		m.ScrollVP.Width = w
	}
	m.ScrollVP.Height = scrollbackH
}

// renderScrollbackContent returns ANSI text for the full scrollback (before viewport clipping).
func (m *Model) renderScrollbackContent() string {
	if len(m.Scrollback) == 0 {
		return metaLineStyle.Render(" (scrollback) " + chat.LandmarkPromptReadyShort)
	}
	var b strings.Builder
	for i, line := range m.Scrollback {
		b.WriteString(m.renderScrollbackEntry(line))
		if i < len(m.Scrollback)-1 {
			if isRolePairLine(line, m.Scrollback[i+1]) {
				b.WriteString("\n\n")
			} else {
				b.WriteString("\n")
			}
		}
	}
	return b.String()
}

// isRolePairLine is true when both lines are user or assistant transcript rows (adds a blank line between turns).
func isRolePairLine(a, b string) bool {
	ra := strings.HasPrefix(a, "You: ") || strings.HasPrefix(a, "Assistant: ")
	rb := strings.HasPrefix(b, "You: ") || strings.HasPrefix(b, "Assistant: ")
	return ra && rb
}

func (m *Model) renderScrollbackEntry(line string) string {
	switch {
	case strings.HasPrefix(line, scrollbackSystemLinePrefix):
		return systemLineStyle.Render(line)
	case strings.HasPrefix(line, "You: "):
		return m.renderUserBlock(strings.TrimPrefix(line, "You: "))
	case strings.HasPrefix(line, "Assistant: "):
		return m.renderAssistantBlock(strings.TrimPrefix(line, "Assistant: "))
	default:
		return metaLineStyle.Render(line)
	}
}

// wrapSystemScrollbackLines prefixes each line for dim “system” rendering (slash output, etc.).
func wrapSystemScrollbackLines(lines []string) []string {
	if len(lines) == 0 {
		return nil
	}
	out := make([]string, len(lines))
	for i, l := range lines {
		out[i] = scrollbackSystemLinePrefix + l
	}
	return out
}

func (m *Model) renderRoleBlock(label string, labelSt, frameSt, plainSt *lipgloss.Style, body string) string {
	lbl := labelSt.Render(label)
	var bodyRendered string
	if m.wantPlain() {
		bodyRendered = trimMDEdges(plainSt.Render(body))
	} else {
		bodyRendered = m.glamRender(body)
	}
	bodyRendered = trimMDEdges(bodyRendered)
	if strings.TrimSpace(ansi.Strip(bodyRendered)) == "" {
		return frameSt.Render(lbl)
	}
	inner := lipgloss.JoinVertical(lipgloss.Left, lbl, indentLines(bodyRendered, "  "))
	return frameSt.Render(inner)
}

func (m *Model) renderUserBlock(body string) string {
	return m.renderRoleBlock("You", &userLabelStyle, &userFrameStyle, &plainUserStyle, body)
}

func (m *Model) renderAssistantBlock(body string) string {
	return m.renderRoleBlock("Assistant", &assistantLabelStyle, &assistantFrameStyle, &plainUserStyle, body)
}

func indentLines(s, prefix string) string {
	if s == "" {
		return ""
	}
	lines := strings.Split(s, "\n")
	for i := range lines {
		if lines[i] != "" {
			lines[i] = prefix + lines[i]
		}
	}
	return strings.Join(lines, "\n")
}

func appendReverseVideoCursor(b *strings.Builder, base *lipgloss.Style, after string) {
	cursorSt := base.Reverse(true)
	if after == "" {
		b.WriteString(cursorSt.Render(" "))
		return
	}
	r, sz := utf8.DecodeRuneInString(after)
	if r == utf8.RuneError && sz == 0 {
		b.WriteString(cursorSt.Render(" "))
	} else {
		b.WriteString(cursorSt.Render(string(after[:sz])))
		b.WriteString(base.Render(after[sz:]))
	}
}

// trimMDEdges removes leading/trailing blank lines from rendered markdown (glamour often emits a leading newline).
func trimMDEdges(s string) string {
	return strings.TrimRight(strings.TrimLeft(s, "\n\r"), "\n\r")
}

func (m *Model) glamRender(src string) string {
	src = strings.TrimSpace(src)
	if src == "" {
		return ""
	}
	r, err := m.mdRenderer()
	if err != nil || r == nil {
		return src
	}
	out, err := r.Render(src)
	if err != nil {
		return src
	}
	return trimMDEdges(out)
}

func (m *Model) mdRenderer() (*glamour.TermRenderer, error) {
	if m.wantPlain() {
		return nil, nil
	}
	ww := max(40, m.Width-6)
	cacheKey := fmt.Sprintf("%d-%t-%t", ww, m.wantNoColor(), m.wantPlain())
	if m.mdRendererCacheKey == cacheKey && m.mdRendererCached != nil {
		return m.mdRendererCached, nil
	}
	opts := []glamour.TermRendererOption{glamour.WithWordWrap(ww)}
	if m.wantNoColor() {
		opts = append(opts, glamour.WithStandardStyle(styles.AsciiStyle))
	} else {
		// Do not use glamour.WithAutoStyle(): it queries the terminal (OSC 10/11) via termenv.
		// While Bubble Tea is reading stdin, the color response can be mistaken for typed text
		// (e.g. "rgb:0000/0000/0000" in the composer). Fixed dark style avoids any probe.
		opts = append(opts, glamour.WithStandardStyle(styles.DarkStyle))
	}
	r, err := glamour.NewTermRenderer(opts...)
	if err != nil {
		return nil, err
	}
	m.mdRendererCached = r
	m.mdRendererCacheKey = cacheKey
	return r, nil
}

// composerBaseStyle matches renderComposerBox (dark panel + light text) so every segment carries
// explicit colors. That avoids ANSI resets after the cursor from stripping the panel background.
func (m *Model) composerBaseStyle() lipgloss.Style {
	if m.wantNoColor() {
		return lipgloss.NewStyle()
	}
	return lipgloss.NewStyle().
		Background(lipgloss.Color("236")).
		Foreground(lipgloss.Color("252"))
}

// buildComposerDisplayLines returns composer rows with "> " prefixes, a visible cursor on the active line,
// and at most maxLines rows (scrolling the window so the cursor line stays visible when needed).
func (m *Model) buildComposerDisplayLines(maxLines int) []string {
	m.clampInputCursor()
	base := m.composerBaseStyle()
	lines := strings.Split(m.Input, "\n")
	start, end := m.visibleComposerLineRange(maxLines)
	cl := m.cursorLineIndex()
	var out []string
	for i := start; i < end; i++ {
		line := lines[i]
		prefix := "> "
		if i != cl {
			out = append(out, base.Render(prefix+line))
			continue
		}
		col := m.cursorColumnBytes(i)
		if col < 0 {
			col = 0
		}
		if col > len(line) {
			col = len(line)
		}
		before := line[:col]
		after := line[col:]
		var b strings.Builder
		bb := base
		b.WriteString(bb.Render(prefix + before))
		appendReverseVideoCursor(&b, &bb, after)
		out = append(out, b.String())
	}
	return out
}

// renderStyledLineWithCursor renders one line with the insertion caret at cursorByte, using the same
// reverse-video rules as the composer (space or first rune of the suffix).
func renderStyledLineWithCursor(base *lipgloss.Style, line string, cursorByte int) string {
	cursorByte = clampStringCursor(line, cursorByte)
	before := line[:cursorByte]
	after := line[cursorByte:]
	var b strings.Builder
	b.WriteString(base.Render(before))
	appendReverseVideoCursor(&b, base, after)
	return b.String()
}

func (m *Model) renderComposerBox(composerLines []string) string {
	content := strings.Join(composerLines, "\n")
	// Style.Width is the bordered block's inner width; borders add 2 cells. Without subtracting,
	// total width exceeds the terminal and the right border is clipped.
	innerW := m.Width - 2
	if innerW < 1 {
		innerW = 1
	}
	st := lipgloss.NewStyle().
		Border(lipgloss.NormalBorder()).
		BorderForeground(lipgloss.Color("240")).
		Padding(0, 1)
	if !m.wantNoColor() {
		st = st.Background(lipgloss.Color("236"))
	}
	return st.Width(innerW).Render(content)
}

// renderCopyHintLine returns the dim footnote under the composer (selection + copy).
func (m *Model) renderCopyHintLine() string {
	return copySelectFootnoteStyle.Width(m.Width).Render(copySelectFootnote)
}

func (m *Model) isViewportScrollKey(msg tea.KeyMsg) bool {
	switch msg.Type {
	case tea.KeyPgUp, tea.KeyPgDown:
		return true
	}
	s := msg.String()
	return s == "pgup" || s == "pgdown"
}
