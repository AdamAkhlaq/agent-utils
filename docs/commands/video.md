# `video`

Download a video by driving [`yt-dlp`](https://github.com/yt-dlp/yt-dlp) and print the path of each saved file on `stdout`, one per line, so the result can be piped straight into the next step.

## Usage

```sh
agent-utils video [-o dir] [-audio] <url>
```

## Flags

| Flag       | Description                                     |
| ---------- | ----------------------------------------------- |
| `-o <dir>` | Output directory (default: current directory).  |
| `-audio`   | Download the audio track only.                  |

Requires `yt-dlp` 2023.06.21 or newer on your `PATH` ([installation](https://github.com/yt-dlp/yt-dlp#installation)); the command fails with a clear message if it's missing. `-audio` extraction, and merging of high-resolution formats, additionally use `ffmpeg`. yt-dlp runs quietly: there is no progress bar, and its errors and warnings go to `stderr`. It also runs with `--ignore-config`, so no `yt-dlp.conf` is read: a config file in an untrusted directory could otherwise make it run commands, and your own config (cookies, proxy, format choices) would make the output depend on the machine. With `-audio` the printed path is the extracted audio file, not the intermediate download; its extension follows the source's audio codec. Unlike every other command, this one talks to the network.

## Examples

```sh
agent-utils video "https://www.youtube.com/watch?v=..."
agent-utils video -audio -o ~/Music "https://www.youtube.com/watch?v=..."   # e.g. ~/Music/Title [id].opus
agent-utils filetype "$(agent-utils video "$URL")"                           # e.g. video/mp4, for a URL that saves one file
```

---

[All commands](../../README.md#commands)
