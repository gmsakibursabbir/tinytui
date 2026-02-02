# TiniTUI

A Production-Ready, Cross-Platform TUI + CLI application for compressing images using the TinyPNG / Tinify Developer API.

![TiniTUI Demo](demo.png)

## Features

- **CLI Mode**: Script-friendly command for pipelines and CI/CD.
- **TUI Mode**: Beautiful terminal interface with file browser, queue management, and real-time progress.
- **Cross-Platform**: Works on Linux, macOS, and Windows.
- **Smart Compression**: Supports PNG, JPG/JPEG, WebP (ignores other files).
- **Safe**: Atomic replacements, history tracking, and error handling.

## Installation

### Quick Install

**Linux & macOS:**

```bash
curl -fsSL https://raw.githubusercontent.com/gmsakibursabbir/tinitui/main/install.sh | sh
```

**Windows (PowerShell):**

```powershell
iwr https://raw.githubusercontent.com/gmsakibursabbir/tinitui/main/install.ps1 -useb | iex
```

### Self-Update

Keep TiniTUI up to date with a single command:

```bash
tinitui update
```

### From Source

```bash
git clone https://github.com/gmsakibursabbir/tinitui.git
cd tinitui
go build -o tinitui .
# Optional: Move to PATH
# sudo mv tinitui /usr/local/bin/
```

## Setup

First run will prompt for your TinyPNG API Key:

```bash
tinitui
```

Or set it via CLI:

```bash
tinitui config set-key <YOUR_API_KEY>
```

## Usage

### TUI Mode

Simply run `tinitui` to open the interactive interface.

- **Browser**: Navigate directories (Enter), toggle file selection (Space), Add to Queue (A).
- **Queue**: Review selected files. Press `R` to run compression.
- **Compress**: Watch progress.
- **History**: View past compressions.

**Keybindings:**

- `A`: Add files (from Browser)
- `R`: Run compression
- `S`: Settings (Not fully implemented in MVP)
- `H`: History
- `Q`: Quit

### CLI Mode

Compress specific files or directories:

```bash
tinytui compress ./images/*.png
```

Pipe files from stdin:

```bash
find . -name "*.jpg" | tinytui compress --stdin
```

Options:

- `--output-dir <dir>`: Save compressed files to specific directory.
- `--suffix <suffix>`: Append suffix to filenames (e.g. `.tiny`).

### History

Export history to CSV:

```bash
tinytui history --csv report.csv
```

## Configuration

Stored in `~/.config/tinytui/config.json`.
History stored in `~/.local/state/tinytui/history.json`.

Permissions are restricted to `0600` for security.


text