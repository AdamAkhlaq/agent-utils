# Security policy

## Supported versions

Security fixes go into the latest release only. Upgrade to it before reporting, if you can.

## Reporting a vulnerability

Please do not report vulnerabilities in public issues or discussions. Report them privately through GitHub's [private vulnerability reporting](https://github.com/AdamAkhlaq/agent-utils/security/advisories/new). Include the version (`agent-utils version`), the exact command, an input that reproduces the problem, and what an attacker gains.

This project is maintained by volunteers, so there is no guaranteed response time; reports are handled on a best-effort basis. You'll get an acknowledgment, a fix in a new release for confirmed issues, and credit in the advisory unless you'd rather stay anonymous.

## Scope

In scope: any way that input to a command (stdin, file arguments, flag values) can make `agent-utils` crash in a way that harms the caller, use memory or time far out of proportion to the size of the input and the work requested, produce output that misrepresents its input, or read or write files the caller didn't name, reach the network, or run other programs, beyond what is documented. Two behaviors are documented: an output file is first written to a temporary file beside it, which is renamed into place or removed; and `video` reaches the network, runs `yt-dlp` (which may run `ffmpeg`), and saves files under names `yt-dlp` chooses. Release artifacts and the build pipeline are in scope too.

Out of scope, because the behavior is documented:

- `jwt-decode` does not verify signatures. It decodes, and its documentation says never to treat the claims as authentic on that basis.
- `cert-decode` inspects certificates and does not validate chains.
- `hash -a md5` and `hash -a sha1` are offered for integrity checks and content addressing, not for security.
- `video` runs [yt-dlp](https://github.com/yt-dlp/yt-dlp) as a subprocess with `--ignore-config`, passing the URL as a single argument after `--`, with no shell involved. Vulnerabilities in yt-dlp or ffmpeg themselves belong with those projects.
