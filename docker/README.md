# Docker Setup for Steam Screenshots

This directory contains Docker configurations for running the Steam Screenshots server and uploader.

## Quick Start

1. Copy the example .env and configuration files in the config directory:
   ```bash
   cp .env.example .env
   cp config/server-config.json.example config/server-config.json
   cp config/upload-config.json.example config/upload-config.json
   ```

2. Edit the configuration files:
   - Update the `ApiKey` in both files with a secure key
   - Leave the `Address`, `ImageDirectory` and `RemoteDirectory` values as they are, these are configured in the `.env` file.
   - If you plan on running the server and uploader on different machines, update the `Server` and `ApiWhiteList` values

3. Edit the .env file:
   ```sh
   # Set this to the port you want to expose the server on.
   STEAM_SCREENSHOTS_PORT=8080

   # Set this to the location where you want the server to save screenshots and game banners.
   STEAM_SCREENSHOTS_APPDATA=./appdata

   # Set this to the location of your server-config.json and/or upload-config.json files
   STEAM_SCREENSHOTS_CONFIG=./config

   # Set this to the steam screenshot directory named 'remote' on the uploads
   # e.g. /home/deck/.local/share/Steam/userdata/<steam_user_id>/760/remote on Steamdeck
   # or C:\Program Files (x86)\Steam\userdata\<steam_user_id>\760\remote on Windows
   STEAM_SCREENSHOTS_REMOTE=/path/to/screenshots/remote/

   ```

4. Build and run the containers:
   ```bash
   docker-compose up -d
   ```

   You can also run the containers individually
   ```bash
   docker compose up server -d
   docker compose up upload -d
   ```

## App Configuration

### Server (config/server-config.json)
- `Address`: The bind address and port (default: ":8080") **(NOTE: leave port as 8080, set STEAM_SCREENSHOTS_PORT in the .env instead)**
- `ImageDirectory`: Where screenshots are stored **(NOTE: don't change this, set STEAM_SCREENSHOTS_APPDATA in the .env instead)**
- `ApiWhitelist`: IP addresses/hostnames allowed to use the API
- `ApiKey`: Shared secret for API authentication

### Uploader (upload-config.json)
- `ServerUrl`: URL to reach the server (default: "http://server:8080")
- `ApiKey`: Must match the server's ApiKey
- `ScreenshotDirectory`: Local directory with screenshots to upload **(NOTE: don't change this, set STEAM_SCREENSHOTS_REMOTE in the .env instead)**
- `Interval`: Number of seconds between directory scans

## Networking

Both services are on the same Docker network (`steam-screenshots`) for internal communication.