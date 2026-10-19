package main

import "github.com/adamakhlaq/agent-utils/v2/internal/cli"

// examples maps each command to the invocations its -h output and the
// commands manifest show. Output, when set, is exactly what the command
// prints; TestExamples runs every example it can to hold that true.
var examples = map[string][]cli.Example{
	"base64": {
		{Command: `printf 'hi' | agent-utils base64`, Output: `aGk=`},
		{Command: `printf 'aGk=' | agent-utils base64 -d`, Output: `hi`},
		{Command: `agent-utils base64 photo.png > photo.b64`},
	},
	"bmp2png": {
		{Command: `agent-utils bmp2png scan.bmp scan.png`},
		{Command: `agent-utils bmp2png < scan.bmp > scan.png`},
	},
	"calc": {
		{Command: `agent-utils calc '0.1 + 0.2'`, Output: `0.3`},
		{Command: `agent-utils calc '2^64'`, Output: `18446744073709551616`},
		{Command: `agent-utils calc '1/3'`, Output: `0.33333333333333333333`},
		{Command: `agent-utils calc -scale 2 '19.99 * 3 * 1.0825'`, Output: `64.92`},
		{Command: `agent-utils calc -json '1/3' | jq -r .fraction`, Output: `1/3`},
	},
	"case": {
		{Command: `printf 'camelCase' | agent-utils case -to snake`, Output: `camel_case`},
		{Command: `printf 'HTTPServer' | agent-utils case -to kebab`, Output: `http-server`},
	},
	"cert-decode": {
		{Command: `agent-utils cert-decode fullchain.pem | jq -r '.[].notAfter'`},
		{Command: `openssl s_client -connect example.com:443 -showcerts </dev/null 2>/dev/null | agent-utils cert-decode`},
	},
	"cidr": {
		{Command: `agent-utils cidr info 10.0.5.1/24 | jq -r .network`, Output: `10.0.5.0/24`},
		{Command: `agent-utils cidr contains 10.0.0.0/16 10.0.42.1`, Output: `true`},
		{Command: `agent-utils cidr split 10.0.0.0/24 26 | tail -1`, Output: `10.0.0.192/26`},
	},
	"color": {
		{Command: `agent-utils color -to hsl '#ff8800'`, Output: `hsl(32, 100%, 50%)`},
		{Command: `agent-utils color -to hex 'rgb(255, 136, 0)'`, Output: `#ff8800`},
		{Command: `agent-utils color -json '#663399' | jq .hsl.h`, Output: `270`},
	},
	"commands": {
		{Command: `agent-utils commands | jq -r '.[].name'`},
		{Command: `agent-utils commands | jq '.[] | select(.name == "hash")'`},
	},
	"cron": {
		{Command: `agent-utils cron -n 1 -from 2026-01-01T00:00:00Z '30 9 * * MON-FRI'`, Output: `2026-01-01T09:30:00Z`},
		{Command: `agent-utils cron -n 1 -from 2026-03-01T00:00:00Z '0 0 29 2 *'`, Output: `2028-02-29T00:00:00Z`},
		{Command: `agent-utils cron -n 1 -z America/New_York -from 2026-03-08 '30 2 * * *'`, Output: `2026-03-08T03:00:00-04:00`},
		{Command: `agent-utils cron -json '0 0 1 * MON' | jq -r .day_match`, Output: `either`},
	},
	"csv2json": {
		{Command: `printf 'name,zip\nJane,01234\n' | agent-utils csv2json | jq -r '.[0].zip'`, Output: `01234`},
		{Command: `agent-utils csv2json -sep ';' european.csv`},
	},
	"filetype": {
		{Command: `agent-utils filetype photo.png`, Output: `image/png`},
		{Command: `agent-utils filetype -json photo.png | jq .width`},
	},
	"gif2png": {
		{Command: `agent-utils gif2png animation.gif first-frame.png`},
	},
	"hash": {
		{Command: `printf 'abc' | agent-utils hash -a md5`, Output: `900150983cd24fb0d6963f7d28e17f72`},
		{Command: `agent-utils hash release.tar.gz`},
		{Command: `agent-utils hash -c "$expected" release.tar.gz && echo verified`},
	},
	"hex": {
		{Command: `printf 'hi' | agent-utils hex`, Output: `6869`},
		{Command: `printf '6869' | agent-utils hex -d`, Output: `hi`},
	},
	"img-resize": {
		{Command: `agent-utils img-resize -max 512 photo.jpg thumb.jpg`},
		{Command: `agent-utils img-resize -width 800 screenshot.png small.png`},
	},
	"jpeg2png": {
		{Command: `agent-utils jpeg2png photo.jpg photo.png`},
	},
	"json-fmt": {
		{Command: `printf '{"b":1,"a":[1,2]}' | agent-utils json-fmt -indent 4`},
		{Command: `agent-utils json-fmt -c data.json`},
		{Command: `agent-utils json-fmt -check data.json || echo invalid`},
	},
	"json2csv": {
		{Command: `printf '[{"name":"Jane","zip":"01234"}]' | agent-utils json2csv`},
		{Command: `agent-utils csv2json data.csv | agent-utils json2csv`},
	},
	"json2toml": {
		{Command: `printf '{"name":"demo","port":8080}' | agent-utils json2toml`},
	},
	"json2yaml": {
		{Command: `printf '{"name":"demo","tags":["a","b"]}' | agent-utils json2yaml`},
	},
	"jwt-decode": {
		{Command: `printf '%s' "$TOKEN" | agent-utils jwt-decode | jq .payload.exp`},
	},
	"lorem": {
		{Command: `agent-utils lorem -w 5`, Output: `Lorem ipsum dolor sit amet,`},
		{Command: `agent-utils lorem -p 3`},
	},
	"md-table": {
		{Command: `printf '[{"name":"Jane","age":30}]' | agent-utils md-table`},
		{Command: `agent-utils md-table -csv report.csv`},
	},
	"password": {
		{Command: `agent-utils password`},
		{Command: `agent-utils password -l 32 -n 5 -no-symbols`},
	},
	"png2jpeg": {
		{Command: `agent-utils png2jpeg -quality 95 in.png out.jpg`},
	},
	"qr": {
		{Command: `agent-utils qr -o qr.png 'https://example.com'`},
		{Command: `agent-utils qr -d qr.png`, Output: `https://example.com`},
	},
	"quote": {
		{Command: `printf '%s' "it's here" | agent-utils quote -as shell`, Output: `'it'"'"'s here'`},
		{Command: `printf 'say "hi"\n' | agent-utils quote -as json`, Output: `"say \"hi\"\n"`},
		{Command: `printf '<a href="/">' | agent-utils quote -as html`, Output: `&lt;a href=&#34;/&#34;&gt;`},
		{Command: `printf 'v1.2.*' | agent-utils quote -as regex`, Output: `v1\.2\.\*`},
		{Command: `printf '%s' '"caf\u00e9"' | agent-utils quote -d -as json`, Output: `café`},
	},
	"semver": {
		{Command: `printf '1.2.0\n1.10.0\n' | agent-utils semver -r | head -1`, Output: `1.10.0`},
		{Command: `agent-utils semver -compare 1.2.3 1.10.0`, Output: `-1`},
		{Command: `agent-utils semver -check '>=1.2.0 <2.0.0' v1.5.3`, Output: `true`},
	},
	"slugify": {
		{Command: `printf 'Hello, World!' | agent-utils slugify`, Output: `hello-world`},
	},
	"strip-ansi": {
		{Command: `printf '\033[31mred\033[0m\n' | agent-utils strip-ansi`, Output: `red`},
		{Command: `agent-utils strip-ansi ci.log clean.log`},
	},
	"time": {
		{Command: `agent-utils time 1755459000`, Output: `2025-08-17T19:30:00Z`},
		{Command: `agent-utils time -z Asia/Tokyo 1755459000`, Output: `2025-08-18T04:30:00+09:00`},
		{Command: `agent-utils time -f unix 2025-08-17T19:30:00Z`, Output: `1755459000`},
		{Command: `agent-utils time add 2024-01-31 P1M`, Output: `2024-02-29T00:00:00Z`},
		{Command: `agent-utils time -z America/New_York add 2024-03-09T12:00:00-05:00 P1D`, Output: `2024-03-10T12:00:00-04:00`},
		{Command: `agent-utils time diff 2024-01-31 2024-03-01T12:00:00Z`, Output: `P1M1DT12H`},
		{Command: `agent-utils time -json diff 2026-01-01 2026-12-25 | jq .total_days`, Output: `358`},
		{Command: `agent-utils time -json 2026-12-25 | jq -r .weekday`, Output: `Friday`},
	},
	"tiff2png": {
		{Command: `agent-utils tiff2png scan.tiff scan.png`},
	},
	"toml2json": {
		{Command: `agent-utils toml2json pyproject.toml | jq -r .project.name`},
	},
	"unicode": {
		{Command: `printf 'e\xcc\x81' | agent-utils unicode | jq -r '[.[].codepoint] | join(" ")'`, Output: `U+0065 U+0301`},
		{Command: `printf 'e\xcc\x81' | agent-utils unicode -nfc | agent-utils unicode | jq -r '.[].codepoint'`, Output: `U+00E9`},
		{Command: `agent-utils unicode -check suspicious.txt | jq .findings`},
	},
	"url": {
		{Command: `printf 'a b&c' | agent-utils url`, Output: `a+b%26c`},
		{Command: `echo 'a+b%26c' | agent-utils url -d`, Output: `a b&c`},
	},
	"urlparse": {
		{Command: `agent-utils urlparse 'https://example.com:8443/x?q=1' | jq -r .port`, Output: `8443`},
	},
	"uuid": {
		{Command: `agent-utils uuid`},
		{Command: `agent-utils uuid -n 5`},
	},
	"version": {
		{Command: `agent-utils version`},
	},
	"video": {
		{Command: `agent-utils video 'https://www.youtube.com/watch?v=...'`},
		{Command: `agent-utils video -audio -o ~/Music 'https://www.youtube.com/watch?v=...'`},
	},
	"webp2png": {
		{Command: `agent-utils webp2png photo.webp photo.png`},
	},
	"xml2json": {
		{Command: `printf '<a id="7"><b>x</b></a>' | agent-utils xml2json | jq -r '.a["@id"]'`, Output: `7`},
	},
	"yaml2json": {
		{Command: `agent-utils yaml2json config.yaml | jq .service.name`},
	},
}
