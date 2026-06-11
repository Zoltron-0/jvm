# jvm

A simple, admin-free Java version manager for Windows using directory symlinks and user‑level `PATH`.

It keeps a list of JDK installations in `%USERPROFILE%\.jvm\config.json`, maintains a symlink at `%USERPROFILE%\Java` to the currently active JDK, and optionally adds `%USERPROFILE%\Java\bin` to your user `PATH` so `java` is always available.

For usage, go to [Usage](#usage)

## Features
- **No admin required** – all changes live in your user profile
- **Symlink-based** – just points `%USERPROFILE%\Java` to the active installation
- **Easy switching** – switch by ID, name or quick‑name
- **PATH setup** – one command ensures `%USERPROFILE%\Java\bin` is in your user `PATH`
- **Simple config** – human‑readable JSON with all your installations

## Requirements
- Windows 10 or 11
- Developer Mode enabled (Settings → Privacy & security → For developers) for symlink creation
- Go 1.20+ (if building from source)

## Installation
1. Clone the repository or download the latest `jvm.exe` from [Releases](https://github.com/Zoltron-0/jvm/releases).
2. Place `jvm.exe` somewhere in your `PATH`, or run it directly.
3. Create `%USERPROFILE%\.jvm\config.json`.

## Configuration
Example `config.json` (place at `%USERPROFILE%\.jvm\config.json`):

```json
{
    "active": 1,
    "installations": [
        {
            "id": 1,
            "name": "OpenJDK Runtime Environment GraalVM CE 22.3.3 (build 11.0.20+8-jvmci-22.3-b22)",
            "qickname": "GraalJDK 11",
            "path": "F:\\Java\\Win\\graalvm-ce-java11-22.3.3"
        },
        {
            "id": 2,
            "name": "OpenJDK Runtime Environment Temurin 17.0.7+7",
            "qickname": "Temurin 17",
            "path": "C:\\Program Files\\Eclipse Adoptium\\jdk-17.0.7.7-hotspot"
        }
    ]
}
```

- `"active"`: ID of the currently active installation.
- `"installations"`: list of JDKs you want to manage.
  - `"id"`: unique number
  - `"name"`: full version string (used for `ls full`)
  - `"qickname"`: short name (used for quick listing and switching)
  - `"path"`: absolute path to the JDK root (the folder that contains `bin`, `lib`, etc.)

## Usage
```
jvm help
```
Displays available commands.

```
jvm ls [quick|full]
```
Lists all installations. Defaults to quick name, use `full` for the complete name.

```
jvm switch <id|name|quickname>
```
Switches the symlink and updates the active ID.  
Examples:
- `jvm switch 1`
- `jvm switch "Temurin 17"`
- `jvm switch "GraalJDK 11"`

```
jvm active [quick|full]
```
Shows the currently active installation.

```
jvm setup
```
Adds `%USERPROFILE%\Java\bin` to your user `PATH`. Run once after installing.

## How it works
- `jvm setup` modifies `HKCU\Environment\Path` (user PATH) – no admin needed.
- `jvm switch` rewrites the config and creates/updates a **directory symlink** at `%USERPROFILE%\Java` pointing to the chosen JDK path.
- The symlink is only replaced if the target is already a symlink; real folders are left untouched.

## Building from source
```powershell
git clone https://github.com/Zoltron-0/jvm.git
cd jvm
go build -o jvm.exe
```

## License
MIT