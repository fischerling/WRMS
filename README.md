# WRMS - Wireless Remote Media System

WRMS is a service offering a interactive playlist consisting of songs from one
or multiple backends.
A backend in the context of WRMS is anything that can be searched for music and
is able to play individual songs.
Each song in the interactive playlist has a rate, which determines its
position in the playlist.
Songs can be up- or downvoted to change their rate similar to how reddit handles
posts.

## Compilation

WRMS is build in go therefore you need a go installation to build WRMS.

To build WRMS run: `go build` in the repository root.

## Usage

To start WRMS and serve clients run `./wrms`

```Usage of ./wrms:
  -backends string
    	music backend to use (default "dummy youtube spotify")
  -loglevel string
    	log level (default "Warning")
  -port int
    	port to listen to (default 8080)
  -serve-music-dir string
    	local music directory to serve
```

When WRMS is running Clients can connect to the computer running WRMS on the
specified port (default 8080).
For example connect to [localhost:8080](htpp://localhost:8080) when you are
running WRMS on your own computer.

## Requirements

* mpv

## Available backends

The backends WRMS should use can be controlled with the `backends` command line
argument.
The default value is `dummy youtube spotify`.

### local

The `local` backend allows WRMS to play songs from a local path.
To serve local songs pass the `-serve-music-dir <path>` flag to WRMS.

### spotify

The `spotify` backend requires a spotify premium account to serve songs from
spotify.
The spotify username and password are provided using the environment variables
`WRMS_SPOTIFY_USER` and `WRMS_SPOTIFY_PASSWORD`.

### youtube

The `youtube` backend uses yt-dlp to search youtube and in combination with
mpv to play the selected youtube videos.

### dummy

The `dummy` backend is only used for debugging and development it can not
actually search for or play songs.

## LICENSE

WRMS is licensed under the terms of the GNU General Public License 3.0.
A copy of the License can be found in the LICENSE file or at
https://www.gnu.org/licenses/gpl-3.0.en.html.
