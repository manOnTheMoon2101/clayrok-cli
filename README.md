# Clayrok

A terminal chat client for the [xAI Grok API](https://docs.x.ai/). Keeps full conversation history within a session, lets you switch models on the fly, and optionally displays your prepaid credit balance in your local currency.

---

## Preview

<video src="assets/clayrok-gif.mp4" autoplay loop muted playsinline></video>

---

## Colour palette

| Swatch | Hex | Used for |
|---|---|---|
| ![#2D132C](https://placehold.co/14x14/2D132C/2D132C.png) | `#2D132C` | Error / warning text, low-credit indicator |
| ![#801336](https://placehold.co/14x14/801336/801336.png) | `#801336` | Borders, separators, medium-credit indicator |
| ![#C72C41](https://placehold.co/14x14/C72C41/C72C41.png) | `#C72C41` | **You** label, command names, input cursor, high-credit indicator |
| ![#EE4540](https://placehold.co/14x14/EE4540/EE4540.png) | `#EE4540` | **Clayrok** label, header, spinner, accent highlights |

Muted body text uses `#a07890` and response content uses `#f5e6f0`.

---

## Stack

| Library | Purpose |
|---|---|
| [huh](https://github.com/charmbracelet/huh) | Input prompt, spinner |
| [bubbletea](https://github.com/charmbracelet/bubbletea) | Interactive command/model picker |
| [lipgloss](https://github.com/charmbracelet/lipgloss) | Colours, borders, layout |
| [godotenv](https://github.com/joho/godotenv) | `.env` file loading |
| [xAI API](https://docs.x.ai/) | Language models (`/v1/chat/completions`, `/v1/language-models`) |
| [open.er-api.com](https://open.er-api.com) | Live currency exchange rates (optional) |

---

## Setup
Explain SEO using a simple analogy.

### 1. Inference API key (required)

Get a key from [console.x.ai](https://console.x.ai/) and add it to `.env`:

```env
XAI_API_KEY=xai-...
GROK_MODEL=grok-3
```

`GROK_MODEL` defaults to `grok-3` if not set.

### 2. Management API key (optional — for credit balance display)

The credit balance shown in the status bar requires a separate **Management API key** with Billing Read permission:

1. Go to [console.x.ai](https://console.x.ai/) → **Settings → Management Keys**
2. Create a key and grant it **Billing Read** permission
3. Find your Team ID in the console URL: `console.x.ai/team/<team-id>/...`

```env
XAI_MANAGEMENT_KEY=xai-mgmt-...
XAI_TEAM_ID=default
```

> Credits are only displayed when you have a prepaid balance. Pay-as-you-go accounts will show nothing in the credits field.

### 3. Display currency (optional)

By default amounts are shown in USD. Set `CLAYROK_CURRENCY` to any ISO 4217 code to convert using a live exchange rate:

```env
CLAYROK_CURRENCY=ZAR
```

Supported symbols out of the box: `USD` `ZAR` `EUR` `GBP` `JPY` `CNY` `CAD` `AUD` `CHF` `INR` `BRL`. Any other valid code will use the code itself as a prefix (e.g. `NOK 42.00`).

---

## `.env` file locations

Clayrok looks in two places, in this order:

1. Current directory (`./.env`)
2. `~/.config/clayrok/.env`

Variables already in your shell environment are never overwritten, so you can override any setting without touching a file.

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

---

## Usage

Type a message and press **Enter** to chat. The conversation history is kept for the whole session so Grok remembers earlier messages.

| Input | Action |
|---|---|
| Any text + Enter | Send a message |
| `/` + Enter | Open the interactive command picker |
| `/clear` | Clear conversation history and start fresh |
| `/help` | List available commands |
| `/model` | Browse and switch the active model |
| `exit` or `quit` | Quit |
| Ctrl+C | Quit |

### Command picker

Typing `/` and pressing Enter opens a keyboard-navigable list of all commands with descriptions:

```
  /clear                Clear the conversation and start a fresh session
  /help                 Show this help message
  /model                List and switch the active model

  esc cancel · ↑↓ navigate · ↵ select
```

### Model switcher

`/model` fetches the live list of chat-capable models from the xAI API and lets you pick one interactively. The selected model is used for all subsequent messages in the session without clearing history.

```
  grok-4.7              131k ctx
  grok-4.7-reasoning    256k ctx
  grok-3                131k ctx
  ...

  esc cancel · ↑↓ navigate · ↵ select
```

### Tab completion

While typing in the input prompt, press **Tab** to autocomplete a command name from the suggestions list.

---

## Debugging

| Env var | Effect |
|---|---|
| `CLAYROK_DEBUG=1` | Print the raw HTTP status and JSON body of the balance API response to stderr |

If the credit balance shows as unavailable or zero unexpectedly, run:

```sh
CLAYROK_DEBUG=1 go run .
```

The raw response will be printed to stderr on startup, which helps identify field-name mismatches.
