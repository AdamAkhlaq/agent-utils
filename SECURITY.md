# Security policy

## Supported versions

Security fixes go into the latest release. Upgrade to it before reporting, if you can.

## Reporting a vulnerability

Please do not report vulnerabilities in public issues or discussions. Report them privately through GitHub's [private vulnerability reporting](https://github.com/AdamAkhlaq/agent-utils/security/advisories/new). Include the version (`agent-utils version`), the exact command, an input that reproduces the problem, and what an attacker gains.

This project is maintained by volunteers, and reports are handled on a best-effort basis. You'll get an acknowledgement, a fix in a new release for confirmed issues, and credit in the advisory unless you'd rather stay anonymous.

## Scope

In scope: any way that input to a command (stdin, file arguments, flag values) can make `agent-utils` crash in a way that harms the caller, read or write files the caller didn't name, reach the network outside `video`, run other programs, or produce output that misrepresents its input. Release artifacts and the build pipeline are in scope too.

Out of scope, because the behavior is documented:

- `jwt-decode` does not verify signatures. It decodes, and its documentation says never to treat the claims as authentic on that basis.
- `cert-decode` inspects certificates and does not validate chains.
- `hash -a md5` and `hash -a sha1` are offered for integrity checks and content addressing, not for security.
- `video` runs [yt-dlp](https://github.com/yt-dlp/yt-dlp) as a subprocess, with the URL passed as a single argument after `--` and no shell involved. Vulnerabilities in yt-dlp or ffmpeg themselves belong with those projects.
