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

### Windows-Native Tool (`.exe`)
Specifically designed for Windows users who want to run the tool by double-clicking it from the File Explorer.
- **Auto-Copy**: Automatically detects your Windows Downloads folder and offers to copy the resulting archive there.
- **Explorer-Friendly**: Includes a "Press Enter to exit" prompt so the window remains open after completion.
- Binary: `buildREFrameworkWin.exe`

## Usage

### Windows (Native Executable)
1. Double-click `buildREFrameworkWin.exe` in the file explorer.
2. Follow the on-screen prompts (Releases to show, version selection, copy to Downloads).
3. The window will stay open until you press Enter.

### Linux/WSL2 (Primary Build Script)
This is the fastest and most efficient way to build using Go.
```bash
./build.sh
```

### Alternative Build Script (Shell-based)
Identical functionality implemented entirely in Bash.
```bash
./build_sh.sh
```

### Silent Mode
Run the entire pipeline without interactive prompts. This will:
- Select the latest nightly release.
- Force a rebuild if the archive already exists.
- Use the discovered Windows user/Downloads folder automatically.
```bash
./build.sh -silent
# OR
./build_sh.sh -silent

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
| **Go (Streaming)** | `./build.sh` | **~1.5s** |
| **Shell (RAM Disk)** | `./build_sh.sh` | **~1.8s** |

*\*Times measured for a forced rebuild and copy operation on WSL2.*
