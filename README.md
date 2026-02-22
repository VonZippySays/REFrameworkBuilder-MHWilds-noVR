# REFrameworkBuilder-MHWilds-noVR

A high-performance build system for REFramework (Monster Hunter Wilds) that automatically downloads, filters, and packages nightly releases while removing VR/XR runtimes and unwanted files.

## Features

- **High Performance**: 
  - **Go Implementation**: Uses Zip-to-Zip transcoding to filter and rebuild archives entirely in memory/streams—**zero disk extraction**.
  - **Shell Implementation**: Optimized with RAM disk (`/dev/shm`) usage and minimal process forks.
- **Selective Filtering**: Automatically removes `REFramework`, `vr`, `xr`, `DELETE`, and `OpenVR/XR` files from the final package.
- **Dynamic Portability**: Automatically discovers Windows users (`/mnt/c/Users/`) and provides interactive destination selection.
- **GitHub API Integration**: Robust ETag caching to avoid rate limits.
- **Archive Summaries**: Displays a detailed content list and file count after every build.

### Windows-Native Tools (`.exe`)
We maintain two versions specifically for Windows users:
- **GUI Version (`buildREFrameworkWinGUI.exe`)**: A stable visual experience using **Zenity native dialogs** and a **real-time progress bar**. No command prompt window appears.
- **CLI Version (`buildREFrameworkWinCLI.exe`)**: A traditional terminal-based version with robust path handling and a "Press Enter to exit" prompt.
- **Auto-Copy**: Both versions automatically detect your Windows Downloads folder and offer to copy the resulting archive there.

## Usage

### Windows (Native Executable)
1. Double-click `buildREFrameworkWin.exe` in the file explorer.
2. Follow the on-screen prompts (Releases to show, version selection, copy to Downloads).
3. The window will stay open until you press Enter.

### Linux/WSL2 (Go binary — fastest)
```bash
./go.sh
```

### Linux/WSL2 (Pure Shell fallback)
```bash
./shell.sh
```

### Silent Mode
Run the entire pipeline without interactive prompts. This will:
- Select the latest nightly release.
- Force a rebuild if the archive already exists.
- Use the discovered Windows user/Downloads folder automatically.
```bash
./go.sh -silent
# OR
./shell.sh -silent

# Windows Native
./buildREFrameworkWin.exe -silent (or set SILENT=1 env var)
```

## Configuration (Optional)

The scripts support several environment variables for advanced users:
- `SILENT=1`: Alternative way to trigger silent mode.
- `MAX_LIST=N`: Number of releases to display in the menu.
- `DEV_PREFIX=N`: Filter nightly versions by numeric prefix.
- `SKIP_DOWNLOAD=1`: Dry-run mode to verify logic and naming without downloading large zips.

## Performance

| Implementation | Tool | Est. Build Time* |
| :--- | :--- | :--- |
| **Go (Streaming)** | `./go.sh` | **~1.5s** |
| **Shell (RAM Disk)** | `./shell.sh` | **~1.8s** |

*\*Times measured for a forced rebuild and copy operation on WSL2.*
