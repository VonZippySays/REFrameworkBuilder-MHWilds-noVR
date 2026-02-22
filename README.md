# REFrameworkBuilder-MHWilds-noVR

A high-performance build system for REFramework (Monster Hunter Wilds) that automatically downloads, filters, and packages nightly releases while removing VR/XR runtimes and unwanted files.

## Features

- **High Performance**:
  - **Go Implementation**: Uses Zip-to-Zip transcoding to filter and rebuild archives entirely in memory/streams — **zero disk extraction**.
  - **Shell Implementation**: Optimized with RAM disk (`/dev/shm`) usage and minimal process forks.
- **Selective Filtering**: Automatically removes `REFramework`, `vr`, `xr`, `DELETE`, and `OpenVR/XR` files from the final package.
- **GitHub API Integration**: Robust ETag caching to avoid rate limits.

### Windows-Native Tools (`.exe`)
Two pre-built executables for Windows users — no install required:
- **GUI Version (`buildREFrameworkWinGUI.exe`)**: Dark-themed Fyne GUI with a real-time progress bar and scrollable version list. No console window.
- **CLI Version (`buildREFrameworkWinCLI.exe`)**: Lightweight terminal-based version.
- **Auto-Copy**: Both versions detect your Windows Downloads folder and offer to copy the result there.

## Usage

### Windows (Native Executable)
Double-click `buildREFrameworkWinGUI.exe` and follow the prompts, or run `buildREFrameworkWinCLI.exe` from a terminal.

### Linux/WSL2 (Go binary — fastest)
```bash
./go.sh
```

### Linux/WSL2 (Pure Shell fallback)
```bash
./shell.sh
```

### Building the Windows Executables (from WSL2/Linux)
Requires `mingw64-gcc` and `upx`. Builds and optimizes all three binaries, then copies to your Windows Downloads folder.
```bash
./build.sh          # builds GUI + CLI + Linux binary
./build.sh gui      # GUI only
./build.sh cli      # CLI only
./build.sh linux    # Linux binary only
```

### Silent Mode
Skips all prompts — picks the latest release, rebuilds if archive exists, auto-copies to Downloads.
```bash
./go.sh -silent
# OR
./shell.sh -silent

# Windows Native
SILENT=1 buildREFrameworkWinCLI.exe
```

## Configuration (Optional)

| Variable | Default | Description |
| :--- | :--- | :--- |
| `SILENT=1` | — | Skip all prompts, pick latest |
| `MAX_LIST=N` | `20` | Number of releases to display |
| `DEV_PREFIX=N` | — | Filter nightly versions by numeric prefix |
| `SKIP_DOWNLOAD=1` | — | Dry-run mode (no download) |

## Performance

| Implementation | Tool | Est. Build Time* |
| :--- | :--- | :--- |
| **Go (Streaming)** | `./go.sh` | **~1.5s** |
| **Shell (RAM Disk)** | `./shell.sh` | **~1.8s** |

*\*Times measured for a forced rebuild on WSL2.*
