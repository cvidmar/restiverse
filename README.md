# Restiverse

A terminal-based REST API client written in Go that bridges the gap between powerful command-line tools and the convenience of a unified interface.

![Restiverse Screenshot](restiverse-screenshot.png)

## Features

- **Terminal-native**: Built for developers who live in the terminal
- **File-based workflow**: Requests are easy to version and response artifacts are easy to script
- **Variable substitution**: Dynamic URLs, headers, and bodies for different environments (dev/staging/prod)
- **Midnight Commander-style navigation**: Familiar and efficient directory browsing
- **Configurable actions**: Execute custom commands on response files
- **Large output friendly**: Response bodies stream directly to files instead of being buffered in memory
- **Zero lock-in**: Uses simple `.http` files and YAML configuration

## Installation

```bash
# Clone the repository
git clone https://github.com/cvidmar/restiverse.git
cd restiverse

# Build the binary
go build -o restiverse ./cmd/restiverse

# Optionally, move to your PATH
sudo mv restiverse /usr/local/bin/
```

## Quick Start

```bash
# Start in current directory
restiverse

# Start in a specific directory
restiverse ~/api-tests

# Enable debug logging (writes restiverse-debug.log)
DEBUG=1 restiverse .

# Help and version
restiverse --help
restiverse --version
```

## Usage

### Creating HTTP Requests

Create `.http` files with the following format:

```http
GET https://api.example.com/users
Accept: application/json
```

POST with JSON body:

```http
POST https://api.example.com/users
Content-Type: application/json

{
  "name": "John Doe",
  "email": "john@example.com"
}
```

Lines starting with `#` are comments and are ignored:

```http
# Fetch the user list
GET https://api.example.com/users
Accept: application/json
```

### Variable Substitution

Use variables in request URLs, headers, and bodies across different environments:

```http
GET https://srv-{node}.{environ}.example.com/api
Authorization: Bearer {token}

{
  "target": "{node}"
}
```

Define available values in `restiverse.yaml` (example):

```yaml
vars:
  environ:
    - stage
    - prod
  node:
    - a
    - b
    - c
  token:
    - staging-token-xyz
    - prod-token-abc
```

**Using variables:**
- Press **v** on any `.http` file to configure variable values
- Navigate through variables and select values with arrow keys
- Values are saved in `.vars` files and remembered for future requests
- The file browser shows current values: `https://srv-{node:a}.{environ:stage}.example.com/api`
- Variables are automatically substituted in URLs, header values, and request bodies

### Navigation

- **Arrow Keys (↑/↓)**: Navigate through files and folders
- **Enter**: Open folder or show actions for file
- **Backspace**: Navigate to parent directory
- **Space**: Multi-select files
- **n**: Create new `.http` file (opens in `$EDITOR`)
- **R**: Rename selected file
- **D**: Duplicate selected `.http` file (asks for a filename, defaults to `NAME-copy.http`)
- **X**: Delete selected file (asks for confirmation)
- **c**: Open the base directory's `restiverse.yaml` in your configured editor
- **v**: Configure variables (when `.http` file with variables is selected)
- **r**: Execute HTTP request (when `.http` file is selected)
- **h**: View response history
- **e**: Edit file in your `$EDITOR`
- **/**: Open fuzzy finder (use arrow keys to navigate results)
- **ESC**: Close modals or cancel the active HTTP request
- **q**: Quit (asks for confirmation)
- **Ctrl+C**: Quit immediately

### Actions Menu

Press **Enter** on a `.http` file, or on a response in the history view, to open the actions menu. It lists every action from `restiverse.yaml` that applies to the current selection, sorted by name, plus a few built-ins:

- **Copy as curl** (`.http` files): copies the request — with variables substituted — to the clipboard as a `curl` command
- **Variables** (`.http` files with variables): opens the variable configuration modal
- **Custom command…** (responses): run a one-off command without adding it to `restiverse.yaml`

**Custom command** opens a text box, substitutes `{filename}` with the selected response body (or `{filename1}`, `{filename2}`, … for multi-selection), and runs it in your shell with full terminal handoff — exactly like a configured action, so pipes and interactive pagers work:

```
jq '.items[] | select(.active)' {filename} | fx
```

Nothing is saved: the next custom command starts from an empty box. Once a one-off proves useful, add it to `restiverse.yaml` as a named action.

### Response Storage

When you execute a request, Restiverse creates a `responses/` folder next to your `.http` file and streams the response into:

- `FILENAME_YYYYMMDD_HHMMSS_mmm.meta` - Request/response metadata (YAML)
- `FILENAME_YYYYMMDD_HHMMSS_mmm.body` - Response body

The millisecond suffix keeps successive single-process executions distinct. Failed requests retain metadata without a body; user-cancelled requests leave no response artifact.

**Automatic Cleanup:** By default, Restiverse keeps only the 5 most recent response files per `.http` file. When you execute a request and save a new response, older responses beyond the limit are automatically deleted. You can configure this with the `max_responses` setting (set to `0` for unlimited).

### Configuration

Restiverse uses the `restiverse.yaml` in the directory where it was started. A default config is created there automatically on first run; configuration files in child directories are not merged. Beyond the actions shown below, the generated default also includes `Copy as curl`, `Rename File`, `Duplicate File`, `Delete File`, `View Meta` and `Delete Response`.

Example configuration:

```yaml
# Request settings
timeout: 30s

# Editor (defaults to $EDITOR)
editor: $EDITOR

# Response history management
max_responses: 5  # Keep only the 5 most recent responses per .http file (0 = unlimited)

# Variable definitions for URL/header/body substitution
vars:
  environ:
    - stage
    - prod
  node:
    - a
    - b
    - c

# Optional additions to the built-in metadata redaction lists
sensitive_headers:
  - x-workspace-token
sensitive_query_params:
  - nonce

# Actions
actions:
  - name: "Execute Request"
    command: "internal:execute"
    keybinding: "r"
    min_files: 1
    max_files: 1
    file_types: ["http"]

  - name: "Edit File"
    command: "$EDITOR {filename}"
    keybinding: "e"
    min_files: 1
    max_files: 1
    file_types: ["http", "body", "meta"]

  - name: "View Output History"
    command: "internal:history"
    keybinding: "h"
    min_files: 1
    max_files: 1
    file_types: ["http"]

  - name: "View Body"
    command: "less {filename}"
    min_files: 1
    max_files: 1
    file_types: ["body"]

  - name: "View as JSON"
    command: "fx {filename}"
    min_files: 1
    max_files: 1
    file_types: ["body"]
```

### Custom Actions

You can define custom actions to process response files using external commands. For example, to view JSON with [fx](https://github.com/antonmedv/fx):

```yaml
actions:
  - name: "View as JSON"
    command: "fx {filename}"
    min_files: 1
    max_files: 1
    file_types: ["body"]
```

Or compare two responses with [jd](https://github.com/josephburnett/jd):

```yaml
actions:
  - name: "Diff Responses"
    command: "jd {filename1} {filename2}"
    min_files: 2
    max_files: 2
    file_types: ["body"]
```

Placeholders:

- `{filename}` - the selected file (or all selected files, space-separated)
- `{filename1}`, `{filename2}`, … - the selected files individually

All paths are shell-quoted before substitution. Commands run via `sh -c`, so pipes and redirection work. During config loading, only the exact `$EDITOR` and `${EDITOR}` tokens are expanded by Restiverse. Other shell variables such as `$1` and `$PATTERN` are preserved for the shell that runs the action.

### Internal Commands

Actions whose `command` starts with `internal:` are handled by Restiverse itself instead of the shell:

| Command | Description |
| --- | --- |
| `internal:execute` | Execute the request (default keybinding `r`) |
| `internal:history` | Open the response history (default keybinding `h`) |
| `internal:variables` | Configure variable values (keybinding `v`) |
| `internal:copy-as-curl` | Copy the request to the clipboard as a `curl` command |
| `internal:custom-command` | Prompt for a one-off shell command |
| `internal:rename` | Rename the selected file (default keybinding `R`) |
| `internal:duplicate` | Duplicate the selected file (default keybinding `D`) |
| `internal:delete` | Delete the selected file, with confirmation (default keybinding `X`) |

Keybindings are optional; actions without one are still available from the actions menu. Conflicting keybindings are rejected at startup.

## Example Workflow

1. Create a directory for your API tests:
   ```bash
   mkdir ~/api-tests
   cd ~/api-tests
   ```

2. Create an `.http` file:
   ```bash
   mkdir -p api/users
   cat > api/users/get-users.http <<EOF
   GET https://jsonplaceholder.typicode.com/users
   Accept: application/json
   EOF
   ```

3. Start Restiverse:
   ```bash
   restiverse .
   ```

4. Navigate to the file and press `r` to execute the request

5. View the response history with `h`

6. View response with `less` or process with custom tools

## Security

New `.meta`, `.body`, and `.vars` artifacts are created with owner-only (`0600`) permissions. Metadata masks a built-in, case-insensitive set of sensitive request and response headers, including Authorization, Proxy-Authorization, Cookie, Set-Cookie, X-Api-Key, X-Auth-Token, and Api-Key. It also redacts common sensitive query parameters such as `access_token`, `token`, `api_key`, `key`, and `signature`; the configuration options above can extend both lists. Resolved variable values are not written to new metadata.

Redaction cannot inspect arbitrary response bodies, which may still contain credentials. Keep `responses/` and `*.vars` out of version control; this repository's `.gitignore` includes both patterns. Review artifacts before sharing them.

Restiverse enforces one in-flight request per TUI. Running multiple Restiverse processes against the same request directory at the same time is unsupported.

## Try it Out

Test files are included in the `test-data/` directory:

```bash
./restiverse test-data
```

Navigate to `api/users/get-users.http` and press `r` to execute!

## Current Implementation Status

✅ **Fully Implemented:**
- 🗂️ **File browser** with nested folder support and MC-style navigation
- 📝 **`.http` file parsing and execution** with all standard HTTP methods
- 💾 **Response storage** with timestamped `.meta` (YAML) and `.body` files
- 📊 **Response history view** with table layout showing status, duration, and size
- ⚙️ **Action system** with configurable actions via `restiverse.yaml`, sorted by name in the menu
- 🧪 **One-off custom commands** on responses, without editing the config
- 📋 **Copy as curl** with variables substituted
- 🗃️ **File management** - rename (`R`), duplicate (`D`) and delete (`X`) `.http` files
- 🔍 **Fuzzy finder** with arrow key navigation (press `/`)
- ✅ **Multi-selection** support (Space key)
- ⏱️ **Request execution** with timeout and cancellation (ESC)
- 🛠️ **External tool integration** with proper terminal handoff
- ⌨️ **Action keybindings** (r, e, h, R, D, X, plus v and c)
- ✏️ **File creation** - press 'n' to create new `.http` files
- 🔐 **Credential masking** for sensitive metadata headers and query parameters
- 🔄 **Variable substitution** for dynamic URLs, headers, and bodies across environments (press 'v')

⏳ **Not Yet Implemented (Future Enhancements):**
- OAuth flows and dynamic token generation
- GraphQL/WebSocket/gRPC support
- Pre-request scripts and response assertions
- Collection runner for batch execution

## Development

```bash
# Run tests
go test ./...

# Build
go build -o restiverse ./cmd/restiverse

# Run with debug logging
DEBUG=1 ./restiverse test-data
```

## Contributing

Contributions are welcome, but hang on a sec, this is still very early stage.

## License

See [LICENSE](LICENSE) for details.

## Acknowledgments

Built with:
- [Bubbletea](https://github.com/charmbracelet/bubbletea) - TUI framework
- [Lipgloss](https://github.com/charmbracelet/lipgloss) - Style library
- [Bubbles](https://github.com/charmbracelet/bubbles) - TUI components

Inspired by tools like Postman, Insomnia, and HTTPie, but designed for terminal lovers who want the power of command-line tools.
