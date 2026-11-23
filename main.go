package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/huh"
	"github.com/charmbracelet/huh/spinner"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/term"
	"github.com/joho/godotenv"
)

const (
	apiURL  = "https://api.x.ai/v1/chat/completions"
	mgmtURL = "https://management-api.x.ai"
)

// ── Palette ────────────────────────────────────────────────────────────────────
var (
	clrDark      = lipgloss.Color("#801336")
	clrPrimary   = lipgloss.Color("#C72C41")
	clrAccent    = lipgloss.Color("#EE4540")
	clrFg        = lipgloss.Color("#f5e6f0")
	clrMuted     = lipgloss.Color("#a07890")
	clrBotBg     = lipgloss.Color("#8B0000")
	clrSectionBg = lipgloss.Color("#180808")
)

// ── Styles ─────────────────────────────────────────────────────────────────────
var (
	headerStyle = lipgloss.NewStyle().Foreground(clrAccent).Bold(true)
	youStyle    = lipgloss.NewStyle().Foreground(clrPrimary).Bold(true)
	botStyle    = lipgloss.NewStyle().Foreground(clrAccent).Bold(true)
	cmdStyle    = lipgloss.NewStyle().Foreground(clrPrimary).Bold(true).Width(16)
	descStyle   = lipgloss.NewStyle().Foreground(clrMuted)
	errStyle    = lipgloss.NewStyle().Foreground(clrDark).Bold(true)
	mutedStyle  = lipgloss.NewStyle().Foreground(clrMuted)

	userBubble = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(clrPrimary).
			Padding(0, 1).
			Foreground(clrFg)

	botBubble = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(clrAccent).
			Foreground(clrFg)
)

// ── Commands ───────────────────────────────────────────────────────────────────
type cmdEntry struct {
	name string
	desc string
}

var cmdEntries = []cmdEntry{
	{"/clear", "Clear the conversation and start a fresh session"},
	{"/config", "View and edit API keys and settings"},
	{"/help", "Show this help message"},
	{"/model", "List and switch the active model"},
}

var commands []string

func init() {
	for _, c := range cmdEntries {
		commands = append(commands, c.name)
	}
}

// ── huh theme ──────────────────────────────────────────────────────────────────
func clayrokTheme() *huh.Theme {
	t := huh.ThemeBase()

	t.Focused.Base = t.Focused.Base.BorderForeground(clrDark)
	t.Focused.Title = lipgloss.NewStyle().Foreground(clrAccent).Bold(true)
	t.Focused.SelectSelector = lipgloss.NewStyle().Foreground(clrPrimary).Bold(true)
	t.Focused.SelectedOption = lipgloss.NewStyle().Foreground(clrAccent).Bold(true)
	t.Focused.Option = lipgloss.NewStyle().Foreground(clrFg)
	t.Focused.Description = lipgloss.NewStyle().Foreground(clrMuted).Italic(true)
	t.Focused.TextInput.Cursor = lipgloss.NewStyle().Foreground(clrAccent)
	t.Focused.TextInput.Placeholder = lipgloss.NewStyle().Foreground(clrMuted)
	t.Focused.TextInput.Text = lipgloss.NewStyle().Foreground(clrFg)
	t.Focused.TextInput.Prompt = lipgloss.NewStyle().Foreground(clrPrimary)

	t.Blurred.Base = t.Blurred.Base.BorderForeground(clrDark)
	t.Blurred.Title = lipgloss.NewStyle().Foreground(clrMuted)
	t.Blurred.TextInput.Text = lipgloss.NewStyle().Foreground(clrMuted)

	return t
}

// ── Generic picker (Bubbletea model) ──────────────────────────────────────────
type pickerEntry struct {
	name string
	desc string
}

type pickerModel struct {
	entries   []pickerEntry
	cursor    int
	selected  string
	cancelled bool
}

func (m pickerModel) Init() tea.Cmd { return nil }

func (m pickerModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "esc":
			m.cancelled = true
			return m, tea.Quit
		case "up", "k":
			if m.cursor > 0 {
				m.cursor--
			}
		case "down", "j":
			if m.cursor < len(m.entries)-1 {
				m.cursor++
			}
		case "enter":
			m.selected = m.entries[m.cursor].name
			return m, tea.Quit
		}
	}
	return m, nil
}

func (m pickerModel) View() string {
	var b strings.Builder
	b.WriteString("\n")
	for i, e := range m.entries {
		if i == m.cursor {
			name := lipgloss.NewStyle().Foreground(clrPrimary).Bold(true).Width(22).Render(e.name)
			desc := lipgloss.NewStyle().Foreground(clrFg).Render(e.desc)
			fmt.Fprintf(&b, "  %s%s\n", name, desc)
		} else {
			name := lipgloss.NewStyle().Foreground(clrMuted).Width(22).Render(e.name)
			desc := lipgloss.NewStyle().Foreground(clrMuted).Render(e.desc)
			fmt.Fprintf(&b, "  %s%s\n", name, desc)
		}
	}
	b.WriteString("\n")
	b.WriteString(mutedStyle.Render("  esc cancel · ↑↓ navigate · ↵ select"))
	b.WriteString("\n")
	return b.String()
}

func runPicker(entries []pickerEntry) string {
	p := tea.NewProgram(pickerModel{entries: entries})
	m, err := p.Run()
	if err != nil {
		return ""
	}
	if pm, ok := m.(pickerModel); ok && !pm.cancelled {
		return pm.selected
	}
	return ""
}

// ── Currency ───────────────────────────────────────────────────────────────────

type currency struct {
	code   string
	symbol string
	rate   float64 // conversion rate from USD (1 USD = rate units)
}

// knownSymbol returns the display symbol for a currency code.
func knownSymbol(code string) string {
	switch code {
	case "USD":
		return "$"
	case "ZAR":
		return "R"
	case "EUR":
		return "€"
	case "GBP":
		return "£"
	case "JPY", "CNY":
		return "¥"
	case "CAD":
		return "C$"
	case "AUD":
		return "A$"
	case "CHF":
		return "Fr"
	case "INR":
		return "₹"
	case "BRL":
		return "R$"
	default:
		return code + " "
	}
}

type erAPIResp struct {
	Result string             `json:"result"`
	Rates  map[string]float64 `json:"rates"`
}

// fetchExchangeRate fetches a live rate from open.er-api.com (free, no key).
func fetchExchangeRate(client *http.Client, from, to string) (float64, error) {
	if from == to {
		return 1.0, nil
	}
	url := fmt.Sprintf("https://open.er-api.com/v6/latest/%s", from)
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return 1.0, err
	}
	resp, err := client.Do(req)
	if err != nil {
		return 1.0, err
	}
	defer resp.Body.Close()

	var out erAPIResp
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return 1.0, err
	}
	rate, ok := out.Rates[to]
	if !ok {
		return 1.0, fmt.Errorf("currency %q not found in exchange rate response", to)
	}
	return rate, nil
}

// setupCurrency reads CLAYROK_CURRENCY and fetches a live rate when needed.
func setupCurrency(client *http.Client) currency {
	code := strings.ToUpper(strings.TrimSpace(os.Getenv("CLAYROK_CURRENCY")))
	if code == "" {
		code = "USD"
	}
	sym := knownSymbol(code)
	if code == "USD" {
		return currency{code: "USD", symbol: sym, rate: 1.0}
	}
	rate, err := fetchExchangeRate(client, "USD", code)
	if err != nil {
		fmt.Fprintf(os.Stderr, "%s\n",
			mutedStyle.Render("currency: could not fetch "+code+" rate, falling back to USD"))
		return currency{code: "USD", symbol: "$", rate: 1.0}
	}
	return currency{code: code, symbol: sym, rate: rate}
}

// ── API types ──────────────────────────────────────────────────────────────────
type message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type chatRequest struct {
	Model    string    `json:"model"`
	Messages []message `json:"messages"`
}

type chatResponse struct {
	Choices []struct {
		Message message `json:"message"`
	} `json:"choices"`
	Error any `json:"error"`
}

func ask(client *http.Client, apiKey, model string, msgs []message) (string, error) {
	body, err := json.Marshal(chatRequest{Model: model, Messages: msgs})
	if err != nil {
		return "", err
	}
	req, err := http.NewRequest(http.MethodPost, apiURL, bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+apiKey)

	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("API returned %s: %s", resp.Status, strings.TrimSpace(string(data)))
	}
	var out chatResponse
	if err := json.Unmarshal(data, &out); err != nil {
		return "", err
	}
	if len(out.Choices) == 0 {
		return "", errors.New("API returned no choices")
	}
	return out.Choices[0].Message.Content, nil
}

// ── Language models ────────────────────────────────────────────────────────────
type langModel struct {
	ID               string   `json:"id"`
	Aliases          []string `json:"aliases"`
	ContextLength    int      `json:"context_length"`
	InputModalities  []string `json:"input_modalities"`
	OutputModalities []string `json:"output_modalities"`
}

type langModelsResp struct {
	Models []langModel `json:"models"`
}

func fetchModels(client *http.Client, apiKey string) []langModel {
	req, err := http.NewRequest(http.MethodGet, "https://api.x.ai/v1/language-models", nil)
	if err != nil {
		return nil
	}
	req.Header.Set("Authorization", "Bearer "+apiKey)

	resp, err := client.Do(req)
	if err != nil || resp.StatusCode != http.StatusOK {
		if resp != nil {
			resp.Body.Close()
		}
		return nil
	}
	defer resp.Body.Close()

	var out langModelsResp
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil
	}

	var chat []langModel
	for _, m := range out.Models {
		hasTextIn, hasTextOut := false, false
		for _, mod := range m.InputModalities {
			if mod == "text" {
				hasTextIn = true
			}
		}
		for _, mod := range m.OutputModalities {
			if mod == "text" {
				hasTextOut = true
			}
		}
		if hasTextIn && hasTextOut {
			chat = append(chat, m)
		}
	}
	return chat
}

func ctxLabel(n int) string {
	if n <= 0 {
		return ""
	}
	if n >= 1000 {
		return fmt.Sprintf("%dk ctx", n/1000)
	}
	return fmt.Sprintf("%d ctx", n)
}

// ── Balance ────────────────────────────────────────────────────────────────────

// extractCents digs a USD-cents integer out of whatever JSON shape the
// xAI management API returns.  Handles:
//   - nested object:  {"val":"-500"} or {"usdCents":"2500"} or {"value":2500}
//   - plain string:   "2500" or "-500"
//   - plain number:   2500
func extractCents(v any) int64 {
	if v == nil {
		return 0
	}
	switch val := v.(type) {
	case map[string]any:
		for _, k := range []string{"val", "usdCents", "usd_cents", "cents", "value", "amount", "usd"} {
			if inner, ok := val[k]; ok {
				if c := extractCents(inner); c != 0 {
					return c
				}
			}
		}
	case string:
		n, _ := strconv.ParseInt(val, 10, 64)
		return n
	case float64:
		return int64(val)
	case json.Number:
		n, _ := val.Int64()
		return n
	}
	return 0
}

// fetchBalance returns (balanceDollars, pct, error).
// pct is -1 when it cannot be computed (no purchase history in the response).
//
// When the response parses to $0 the raw JSON is written to
// /tmp/clayrok_balance.json so you can inspect it.
// Set CLAYROK_DEBUG=1 to always dump it.
func fetchBalance(client *http.Client, mgmtKey, teamID string) (float64, float64, error) {
	url := fmt.Sprintf("%s/v1/billing/teams/%s/prepaid/balance", mgmtURL, teamID)
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return 0, -1, err
	}
	req.Header.Set("Authorization", "Bearer "+mgmtKey)

	resp, err := client.Do(req)
	if err != nil {
		return 0, -1, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return 0, -1, err
	}

	debug := os.Getenv("CLAYROK_DEBUG") == "1"
	if debug {
		fmt.Fprintf(os.Stderr, "[debug] balance status: %s\n[debug] balance body:\n%s\n",
			resp.Status, body)
	}

	if resp.StatusCode != http.StatusOK {
		return 0, -1, fmt.Errorf("balance API %s: %s", resp.Status, strings.TrimSpace(string(body)))
	}

	var raw map[string]any
	if err = json.Unmarshal(body, &raw); err != nil {
		return 0, -1, fmt.Errorf("balance parse: %w", err)
	}

	// ── Try to find the balance amount ─────────────────────────────────────
	// Only check specific, trusted field names.  Broad "scan everything"
	// fallbacks are dangerous — they pick up spend transactions whose values
	// are negative or unrelated, producing nonsense like -R169.
	var totalCents int64
	for _, k := range []string{
		"total", "balance", "currentBalance", "prepaidBalance",
		"remaining", "creditBalance", "availableCredits",
	} {
		if v, ok := raw[k]; ok {
			totalCents = extractCents(v)
			if debug {
				fmt.Fprintf(os.Stderr, "[debug] key %q → %d cents\n", k, totalCents)
			}
			if totalCents != 0 {
				break
			}
		}
	}

	// A negative prepaid balance means the user is on a postpaid plan or has
	// exhausted credits — clamp to zero so the display is clean.
	if totalCents < 0 {
		totalCents = -totalCents // xAI stores amounts as negative (money-out convention)
	}

	// If we still got zero and are not in debug mode, save the raw body so it
	// can be inspected.
	if totalCents == 0 && !debug {
		debugPath := filepath.Join(os.TempDir(), "clayrok_balance.json")
		_ = os.WriteFile(debugPath, body, 0600)
		fmt.Fprintf(os.Stderr, "%s\n%s\n",
			mutedStyle.Render("balance: could not parse amount from API response."),
			mutedStyle.Render("Raw JSON saved to: "+debugPath+" — share it to fix the parser."))
	}

	dollars := float64(totalCents) / 100.0

	// Sum all PURCHASE-type changes to derive a remaining percentage.
	// The API uses "changeOrigin" (not "type") and stores amounts as negative.
	var purchasedCents int64
	if changes, ok := raw["changes"].([]any); ok {
		for _, change := range changes {
			cm, ok := change.(map[string]any)
			if !ok {
				continue
			}
			// "changeOrigin" can be "PURCHASE" or "AUTO_PURCHASE"
			origin, _ := cm["changeOrigin"].(string)
			if strings.Contains(strings.ToUpper(origin), "PURCHASE") {
				c := extractCents(cm["amount"])
				if c < 0 {
					c = -c
				}
				purchasedCents += c
			}
		}
	}

	pct := -1.0
	if purchasedCents > 0 {
		pct = float64(totalCents) / float64(purchasedCents) * 100
	}
	return dollars, pct, nil
}

func formatCredits(dollars, pct float64, cur currency) string {
	amount := dollars * cur.rate
	bal := lipgloss.NewStyle().Foreground(clrFg).Bold(true).
		Render(fmt.Sprintf("%s%.2f", cur.symbol, amount))
	if pct > 0 {
		var pctColor lipgloss.Color
		switch {
		case pct > 50:
			pctColor = clrAccent
		case pct > 20:
			pctColor = clrPrimary
		default:
			pctColor = clrDark
		}
		pctStr := lipgloss.NewStyle().Foreground(pctColor).
			Render(fmt.Sprintf("(%.0f%%)", pct))
		return bal + " " + pctStr
	}
	return bal
}

// ── App state ──────────────────────────────────────────────────────────────────
type appState struct {
	model     string
	creditStr string
	history   []message
}

func printStatus(state *appState) {
	// Thin separator makes the status bar visually distinct from the conversation.
	fmt.Println(mutedStyle.Render(strings.Repeat("─", 48)))
	line := mutedStyle.Render("model: ") + headerStyle.Render(state.model)
	if state.creditStr != "" {
		line += mutedStyle.Render("  ·  credits: ") + state.creditStr
	}
	fmt.Println(line)
}

func printUserMsg(input string) {
	fmt.Printf("\n%s\n%s\n",
		youStyle.Render("You"),
		userBubble.Render(input))
}

// getTermWidth tries all three standard fds and falls back to 80.
func getTermWidth() int {
	for _, fd := range []uintptr{os.Stdin.Fd(), os.Stdout.Fd(), os.Stderr.Fd()} {
		if w, _, err := term.GetSize(fd); err == nil && w > 0 {
			return w
		}
	}
	return 80
}

func printBotMsg(name, reply string) {
	w := getTermWidth()

	nameLabel := lipgloss.NewStyle().
		Foreground(clrAccent).
		Bold(true).
		Render(name)

	// Width: terminal - border(2) - padding(4) = w-6
	section := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(clrDark).
		Background(clrSectionBg).
		Width(w-6).
		Padding(1, 2).
		Foreground(clrFg).
		Render(nameLabel + "\n\n" + reply)

	fmt.Printf("\n%s\n\n", section)
}

// ── Env / setup ────────────────────────────────────────────────────────────────
func loadEnv() {
	_ = godotenv.Load()
	if dir, err := os.UserConfigDir(); err == nil {
		_ = godotenv.Load(filepath.Join(dir, "clayrok", ".env"))
	}
}

// ── Commands ───────────────────────────────────────────────────────────────────
func showCommands() {
	fmt.Println()
	for _, c := range cmdEntries {
		fmt.Printf("  %s%s\n", cmdStyle.Render(c.name), descStyle.Render(c.desc))
	}
	fmt.Printf("  %s%s\n",
		lipgloss.NewStyle().Foreground(clrMuted).Width(16).Render("exit"),
		descStyle.Render("Quit the application"))
	fmt.Println()
}

func pickModel(state *appState, models []langModel) {
	if len(models) == 0 {
		fmt.Println(mutedStyle.Render("No models available — check your API key."))
		return
	}
	entries := make([]pickerEntry, len(models))
	for i, m := range models {
		entries[i] = pickerEntry{name: m.ID, desc: ctxLabel(m.ContextLength)}
	}
	selected := runPicker(entries)
	if selected != "" && selected != state.model {
		state.model = selected
		fmt.Printf("%s %s\n",
			mutedStyle.Render("Switched to"),
			headerStyle.Render(selected))
	}
}

func runCommand(cmd string, state *appState, systemMsg message, models []langModel, shortClient *http.Client) {
	switch cmd {
	case "/clear":
		state.history = []message{systemMsg}
		fmt.Println(headerStyle.Render("✓ Conversation cleared. Starting fresh."))
	case "/help":
		showCommands()
	case "/model":
		pickModel(state, models)
	case "/config":
		runConfig(state, shortClient)
	default:
		fmt.Println(errStyle.Render("Unknown command: " + cmd))
	}
}

// ── Config ─────────────────────────────────────────────────────────────────────

func runConfig(state *appState, shortClient *http.Client) {
	configDir, err := os.UserConfigDir()
	if err != nil {
		fmt.Println(errStyle.Render("Could not find config directory: " + err.Error()))
		return
	}
	configPath := filepath.Join(configDir, "clayrok", ".env")

	fmt.Println()
	fmt.Println(headerStyle.Render("Configuration"))
	fmt.Println(mutedStyle.Render("Settings are saved to: " + configPath))
	fmt.Println()

	// Pre-fill with current values so the user can see and edit them.
	newAPIKey := os.Getenv("XAI_API_KEY")
	newModel := os.Getenv("GROK_MODEL")
	if newModel == "" {
		newModel = "grok-3"
	}
	newMgmtKey := os.Getenv("XAI_MANAGEMENT_KEY")
	newTeamID := os.Getenv("XAI_TEAM_ID")
	if newTeamID == "" {
		newTeamID = "default"
	}
	newCurrency := os.Getenv("CLAYROK_CURRENCY")
	if newCurrency == "" {
		newCurrency = "USD"
	}

	form := huh.NewForm(
		huh.NewGroup(
			huh.NewNote().
				Title("Required").
				Description("Your xAI inference key — used for all chat requests.\nGet one at console.x.ai → API Keys."),
			huh.NewInput().
				Title("XAI_API_KEY").
				Description("Required for Clayrok to work at all.").
				Placeholder("xai-...").
				EchoMode(huh.EchoModePassword).
				Value(&newAPIKey),
			huh.NewInput().
				Title("GROK_MODEL").
				Description("Default model to use (you can switch any time with /model).").
				Placeholder("grok-3").
				Value(&newModel),
		),
		huh.NewGroup(
			huh.NewNote().
				Title("Credit balance display (optional)").
				Description(
					"These two keys enable the credit balance shown in the status bar.\n\n"+
						"1. Go to console.x.ai → Settings → Management Keys\n"+
						"2. Create a key with Billing Read permission\n"+
						"3. Paste it below\n\n"+
						"Your Team ID is in the console URL: /team/<id>/..."),
			huh.NewInput().
				Title("XAI_MANAGEMENT_KEY").
				Description("Management key with Billing Read permission.").
				Placeholder("xai-mgmt-...").
				EchoMode(huh.EchoModePassword).
				Value(&newMgmtKey),
			huh.NewInput().
				Title("XAI_TEAM_ID").
				Description("From the console URL — leave as 'default' if unsure.").
				Placeholder("default").
				Value(&newTeamID),
		),
		huh.NewGroup(
			huh.NewNote().
				Title("Display currency (optional)").
				Description("Credit amounts are shown in USD by default.\nSet any ISO 4217 code to convert using live rates.\nExamples: ZAR, EUR, GBP, JPY, CAD, AUD, INR, BRL"),
			huh.NewInput().
				Title("CLAYROK_CURRENCY").
				Description("ISO 4217 currency code.").
				Placeholder("USD").
				Value(&newCurrency),
		),
	).WithTheme(clayrokTheme()).WithShowHelp(false)

	if err := form.Run(); err != nil {
		fmt.Println(mutedStyle.Render("Config cancelled."))
		return
	}

	// Apply defaults for optional fields left blank.
	if newModel == "" {
		newModel = "grok-3"
	}
	if newTeamID == "" {
		newTeamID = "default"
	}
	if newCurrency == "" {
		newCurrency = "USD"
	}

	// Build .env content.
	var buf strings.Builder
	writeLine := func(k, v string) {
		if v != "" {
			fmt.Fprintf(&buf, "%s=%s\n", k, v)
		}
	}
	writeLine("XAI_API_KEY", newAPIKey)
	writeLine("GROK_MODEL", newModel)
	writeLine("XAI_MANAGEMENT_KEY", newMgmtKey)
	writeLine("XAI_TEAM_ID", newTeamID)
	writeLine("CLAYROK_CURRENCY", newCurrency)

	// Ensure the directory exists.
	if err := os.MkdirAll(filepath.Join(configDir, "clayrok"), 0700); err != nil {
		fmt.Println(errStyle.Render("Could not create config directory: " + err.Error()))
		return
	}
	if err := os.WriteFile(configPath, []byte(buf.String()), 0600); err != nil {
		fmt.Println(errStyle.Render("Could not save config: " + err.Error()))
		return
	}

	fmt.Printf("\n%s %s\n",
		headerStyle.Render("✓ Saved to"),
		mutedStyle.Render(configPath))

	// Update in-memory env so the rest of the session benefits immediately.
	os.Setenv("XAI_API_KEY", newAPIKey)
	os.Setenv("GROK_MODEL", newModel)
	os.Setenv("XAI_MANAGEMENT_KEY", newMgmtKey)
	os.Setenv("XAI_TEAM_ID", newTeamID)
	os.Setenv("CLAYROK_CURRENCY", newCurrency)

	// Update active model in state if it changed.
	if newModel != "" && newModel != state.model {
		state.model = newModel
		fmt.Printf("%s %s\n",
			mutedStyle.Render("Model updated to"),
			headerStyle.Render(newModel))
	}

	// Re-fetch credits immediately so the status bar updates without a restart.
	if newMgmtKey != "" {
		cur := setupCurrency(shortClient)
		dollars, pct, err := fetchBalance(shortClient, newMgmtKey, newTeamID)
		if err != nil {
			state.creditStr = errStyle.Render("unavailable")
		} else if dollars > 0 {
			state.creditStr = formatCredits(dollars, pct, cur)
			fmt.Printf("%s %s\n",
				mutedStyle.Render("Credits:"),
				state.creditStr)
		}
	} else {
		state.creditStr = ""
	}
}

// ── Main ───────────────────────────────────────────────────────────────────────
func main() {
	loadEnv()

	apiKey := os.Getenv("XAI_API_KEY")
	if apiKey == "" || apiKey == "your-xai-api-key-here" {
		fmt.Fprintln(os.Stderr, errStyle.Render("Set XAI_API_KEY in .env or ~/.config/clayrok/.env"))
		os.Exit(1)
	}

	modelName := os.Getenv("GROK_MODEL")
	if modelName == "" {
		modelName = "grok-3"
	}

	mgmtKey := os.Getenv("XAI_MANAGEMENT_KEY")
	teamID := os.Getenv("XAI_TEAM_ID")
	if teamID == "" {
		teamID = "default"
	}

	shortClient := &http.Client{Timeout: 10 * time.Second}
	longClient := &http.Client{Timeout: 120 * time.Second}

	// Set up display currency (CLAYROK_CURRENCY=ZAR, EUR, etc.; defaults to USD).
	cur := setupCurrency(shortClient)

	systemMsg := message{Role: "system", Content: "You are a helpful Gen Z assistant, use slang where you can and you may use curse words also :) Always give your complete answer in a single response — never say you will check, look something up, or come back with more info. Give your best, most complete answer immediately. If you are unsure about something, say so and answer anyway."}
	state := &appState{
		model:   modelName,
		history: []message{systemMsg},
	}

	models := fetchModels(shortClient, apiKey)

	if mgmtKey != "" {
		dollars, pct, err := fetchBalance(shortClient, mgmtKey, teamID)
		if err != nil {
			state.creditStr = errStyle.Render("unavailable")
			fmt.Fprintf(os.Stderr, "%s %s\n",
				mutedStyle.Render("credits:"),
				errStyle.Render(err.Error()))
		} else if dollars > 0 {
			// Only show when there is an actual prepaid balance.
			state.creditStr = formatCredits(dollars, pct, cur)
		}
	}

	fmt.Printf("%s %s\n",
		headerStyle.Render("Clayrok"),
		mutedStyle.Render("— type 'exit' or Ctrl+C to quit."))
	fmt.Println(mutedStyle.Render("Tip: type / to browse commands."))
	fmt.Println()

	km := huh.NewDefaultKeyMap()
	km.Input.AcceptSuggestion = key.NewBinding(key.WithKeys("tab"), key.WithHelp("tab", "complete"))
	km.Input.Next = key.NewBinding(key.WithKeys("enter"), key.WithHelp("enter", "next"))

	for {
		printStatus(state)

		var input string
		inputField := huh.NewInput().
			Title("You").
			Suggestions(commands).
			Value(&input)
		err := huh.NewForm(huh.NewGroup(inputField)).
			WithTheme(clayrokTheme()).
			WithKeyMap(km).
			WithShowHelp(false).
			Run()
		if err != nil {
			break
		}
		input = strings.TrimSpace(input)
		if input == "" {
			continue
		}
		if input == "exit" || input == "quit" {
			fmt.Println(mutedStyle.Render("Goodbye."))
			break
		}

		if input == "/" {
			entries := make([]pickerEntry, len(cmdEntries))
			for i, c := range cmdEntries {
				entries[i] = pickerEntry{name: c.name, desc: c.desc}
			}
			if selected := runPicker(entries); selected != "" {
				runCommand(selected, state, systemMsg, models, shortClient)
			}
			continue
		}

		if strings.HasPrefix(input, "/") {
			var known bool
			for _, cmd := range commands {
				if cmd == input {
					known = true
					runCommand(input, state, systemMsg, models, shortClient)
					break
				}
			}
			if !known {
				entries := make([]pickerEntry, len(cmdEntries))
				for i, c := range cmdEntries {
					entries[i] = pickerEntry{name: c.name, desc: c.desc}
				}
				if selected := runPicker(entries); selected != "" {
					runCommand(selected, state, systemMsg, models, shortClient)
				}
			}
			continue
		}

		// ── Normal chat ────────────────────────────────────────────────────────
		printUserMsg(input)

		state.history = append(state.history, message{Role: "user", Content: input})

		var reply string
		var reqErr error
		action := func() { reply, reqErr = ask(longClient, apiKey, state.model, state.history) }
		if err := spinner.New().
			Style(lipgloss.NewStyle().Foreground(clrAccent)).
			TitleStyle(lipgloss.NewStyle().Foreground(clrMuted)).
			Title("Thinking...").
			Action(action).
			Run(); err != nil {
			break
		}
		if reqErr != nil {
			fmt.Fprintln(os.Stderr, errStyle.Render("Error: ")+reqErr.Error())
			state.history = state.history[:len(state.history)-1]
			continue
		}

		state.history = append(state.history, message{Role: "assistant", Content: reply})
		printBotMsg("Clayrok", reply)
	}
}
