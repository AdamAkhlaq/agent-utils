# `filetype`

Identify a file's MIME type from its content and, for images, its dimensions, so a file can be inspected without reading its contents into context.

## Usage

```sh
agent-utils filetype [-json] [file]
```

## Flags

| Flag    | Description                                                             |
| ------- | ----------------------------------------------------------------------- |
| `-json` | Print a JSON object with `mime`, `bytes`, and image `width`/`height`.   |

Detection uses the file's magic bytes (the first 512), never the filename, so a mislabeled file is still identified correctly. The whole input is read: the byte count needs it, and image dimensions can sit deep in the file (progressive JPEGs). `width` and `height` appear only for decodable PNG, JPEG, GIF, WebP, BMP, and TIFF images; unrecognized content reports `application/octet-stream`.

## Examples

```sh
agent-utils filetype photo.png                    # image/png
agent-utils filetype -json photo.png              # pretty-printed {"mime", "bytes", "width", "height"}
curl -s https://example.com/asset | agent-utils filetype
agent-utils filetype -json photo.png | jq -r .mime
```

---

[All commands](../../README.md#commands)
