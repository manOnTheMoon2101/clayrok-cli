# Clayrok

A terminal chat client for the [xAI Grok API](https://docs.x.ai/). Keeps full conversation history within a session, lets you switch models on the fly, and displays your credit usage as a percentage. Web search is enabled by default on every message.

---

## Preview

![Clayrok demo](assets/clayrok.gif)

---

## Colour palette

| Swatch | Hex | Used for |
|---|---|---|
| ![#2D132C](https://placehold.co/14x14/2D132C/2D132C.png) | `#801336` | Error / warning text, section border |
| ![#C72C41](https://placehold.co/14x14/C72C41/C72C41.png) | `#C72C41` | **You** label, command names, input cursor |
| ![#EE4540](https://placehold.co/14x14/EE4540/EE4540.png) | `#EE4540` | **Clayrok** label, header, spinner, accent highlights |
| ![#180808](https://placehold.co/14x14/180808/180808.png) | `#180808` | Clayrok response section background |

Muted body text uses `#a07890` and response content uses `#f5e6f0`.

---

## Stack

| Library | Purpose |
|---|---|
| [huh](https://github.com/charmbracelet/huh) | Input prompt, spinner, config form |
| [bubbletea](https://github.com/charmbracelet/bubbletea) | Interactive command/model picker |
| [lipgloss](https://github.com/charmbracelet/lipgloss) | Colours, borders, layout |
| [godotenv](https://github.com/joho/godotenv) | `.env` file loading |
| [xAI Responses API](https://docs.x.ai/) | Language models with live web search |
| [xAI Management API](https://docs.x.ai/) | Credit balance (optional) |

---

## Setup

### 1. Inference API key (required)

Get a key from [console.x.ai](https://console.x.ai/) and add it to `.env`:

```env
XAI_API_KEY=xai-...
GROK_MODEL=grok-3
```

`GROK_MODEL` defaults to `grok-3` if not set.

### 2. Management API key (optional — for credit usage display)

The credit usage shown in the status bar requires a separate **Management API key** with Billing Read permission:

1. Go to [console.x.ai](https://console.x.ai/) → **Settings → Management Keys**
2. Create a key and grant it **Billing Read** permission
3. Find your Team ID in the console URL: `console.x.ai/team/<team-id>/...`

```env
XAI_MANAGEMENT_KEY=xai-mgmt-...
XAI_TEAM_ID=default
```

> Credit usage is shown as a percentage of consumed credits (e.g. `0% used` when fully topped up). Only displayed when a prepaid balance is detected.

---

## `.env` file locations

Clayrok looks in two places, in this order:

1. Current directory (`./.env`)
2. `~/.config/clayrok/.env`

Variables already in your shell environment are never overwritten. The `/config` command writes directly to `~/.config/clayrok/.env`, so the installed binary always finds its config regardless of where you run it.

---

## Run

```sh
go run .
```

## Install

```sh
go install .
```

This puts the `clayrok` binary in your Go bin directory (`$GOBIN` / `$GOPATH/bin`). Make sure that directory is on `$PATH`, then run it from anywhere:

```sh
clayrok
```

After making code changes, reinstall with `go install .` to update the binary.

---

## Usage

Type a message and press **Enter** to chat. Conversation history is kept for the whole session. Web search is on by default — Grok can pull live data on every message.

| Input | Action |
|---|---|
| Any text + Enter | Send a message |
| `/` + Enter | Open the interactive command picker |
| `/clear` | Clear conversation history and start fresh |
| `/config` | Edit API keys and settings interactively |
| `/help` | List available commands |
| `/model` | Browse and switch the active model |
| `exit` or `quit` | Quit |
| Ctrl+C | Quit |

### `/config` command

Run `/config` to open a guided multi-step form for setting up your environment. Changes are saved to `~/.config/clayrok/.env` and take effect immediately without restarting.

The form covers:

- **Required** — `XAI_API_KEY` and `GROK_MODEL`
- **Credit balance** — `XAI_MANAGEMENT_KEY` and `XAI_TEAM_ID`, with step-by-step instructions
- *(No restart needed — credits refresh live after saving)*

### Command picker

Typing `/` and pressing Enter opens a keyboard-navigable list of all commands:

```
  /clear            Clear the conversation and start a fresh session
  /config           View and edit API keys and settings
  /help             Show this help message
  /model            List and switch the active model

  esc cancel · ↑↓ navigate · ↵ select
```

### Model switcher

`/model` fetches the live list of chat-capable models from the xAI API and lets you pick one interactively. The selected model is used for all subsequent messages without clearing history.

### Tab completion

While typing in the input prompt, press **Tab** to autocomplete a command name.

---

## Web search

Web search is enabled on every message via the [xAI Responses API](https://docs.x.ai/developers/tools/web-search). Grok automatically searches the web when it needs current information — no extra configuration required.

---

## Debugging

| Env var | Effect |
|---|---|
| `CLAYROK_DEBUG=1` | Print the raw HTTP status and JSON body of the balance API response to stderr |

If the credit usage shows as unavailable, run:

```sh
CLAYROK_DEBUG=1 go run .
```
