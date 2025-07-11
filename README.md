# Steam Screenshot Server

A simple little server to host screenshots taken from steam games.

## Configuration

```json
{
    "Address": ":8080",
    "ApiKey": "",
    "ApiWhitelist": ["127.0.0.1"],
    "AppidOverrides": [
        {
            "id": "231410",
            "name": "Kerbal Space Program Demo"
        }, {
            "id": "33440",
            "name": "Driver San Francisco"
        }
    ],
    "ImageDirectory": "/opt/SteamScreenshots/screenshots/"
}
```

The `Address` setting is simply the address to listen on.  For example, if you
only want to allow local connections on port 8080 set this to `127.0.0.1:8080`.

`AppidOverrides` is a list of id's and names to override a game's name.

If the `ApiKey` field is empty, a new key will be generated on each launch of
the server.  The key is printed to STDOUT upon server startup.  You'll need to
manually save this key to the configuration file to have it persist.

`ImageDirectory` is the storage location for all the screenshots.  This folder
must exist.  Unlike previous versions, the derectory structure does *not* mimic
Steam's directory structure.  Each folder inside is named with an appid and
contains the screenshots directly instead of having another `screenshots`
subfolder.

## Recommended Setup

It's recommended to run the server behind a reverse proxy like nginx.  The
screenshot server doesn't implement TLS so this will need to be provided by the
reverse proxy.

See the `systemd` directory in this repo for example Systemd unit files for
both the server and the uploader.

## Uploader

The uploader is meant to be run on any machine that has Steam locally
installed.  There is an example Systemd unit file in the `systemd` folder.

### Example configuration

```json
{
    "Server": "http://127.0.0.1:8080",
    "Key": "",
    "RemoteDirectory": "/path/to/steam/remote/folder/",
    "Interval": 60
}
```

The remote directory can be found by clicking on the "Show on disk" button in
the Steam Screenshot Manager window.  The path entered into the configuration
should be an absolute path ending in `remote/`.
The `Interval` value is the number of seconds between scans, setting it to zero will cause the uploader to exit after a single pass.

## Notes

 * Game names are matched to their appropriate appid using the Steam store API.
 The appid's are cached locally.
 * Game grid icons are also retrieved from steam's servers and cached locally.
 Non-Steam games will use a default "unknown" image.

## Docker
  See the [docker/README.md](docker/README.md) for docker instructions.

## TODO

 * Some sort of install script or pacakge building script

## License

 Steam-screenshots is licensed under the MIT license.  See LICENSE.txt for the
 full text.

 PhotoSwipe is also licensed under the MIT license.  PhotoSwipe can be found
 [here](https://github.com/dimsemenov/photoswipe).

