package main

// examples maps each command to the invocations its -h output and the
// commands manifest show. A trailing "# comment" states the output, so every
// stated result must be what the binary actually prints.
var examples = map[string][]string{
	"base64": {
		`printf 'hi' | agent-utils base64            # aGk=`,
		`printf 'aGk=' | agent-utils base64 -d       # hi`,
		`agent-utils base64 photo.png > photo.b64`,
	},
	"bmp2png": {
		`agent-utils bmp2png scan.bmp scan.png`,
		`agent-utils bmp2png < scan.bmp > scan.png`,
	},
	"case": {
		`printf 'camelCase' | agent-utils case -to snake    # camel_case`,
		`printf 'HTTPServer' | agent-utils case -to kebab   # http-server`,
	},
	"cert-decode": {
		`agent-utils cert-decode fullchain.pem | jq -r '.[].notAfter'`,
		`openssl s_client -connect example.com:443 -showcerts </dev/null 2>/dev/null | agent-utils cert-decode`,
	},
	"cidr": {
		`agent-utils cidr info 10.0.5.1/24 | jq -r .network   # 10.0.5.0/24`,
		`agent-utils cidr contains 10.0.0.0/16 10.0.42.1      # true`,
		`agent-utils cidr split 10.0.0.0/24 26                # 10.0.0.0/26 ... 10.0.0.192/26`,
	},
	"color": {
		`agent-utils color -to hsl '#ff8800'          # hsl(32, 100%, 50%)`,
		`agent-utils color -to hex 'rgb(255, 136, 0)'  # #ff8800`,
		`agent-utils color -json '#663399' | jq .hsl.h  # 270`,
	},
	"commands": {
		`agent-utils commands | jq -r '.[].name'`,
		`agent-utils commands | jq '.[] | select(.name == "hash")'`,
	},
	"csv2json": {
		`printf 'name,zip\nJane,01234\n' | agent-utils csv2json | jq -r '.[0].zip'   # 01234`,
		`agent-utils csv2json -sep ';' european.csv`,
	},
	"filetype": {
		`agent-utils filetype photo.png                   # image/png`,
		`agent-utils filetype -json photo.png | jq .width`,
	},
	"gif2png": {
		`agent-utils gif2png animation.gif first-frame.png`,
	},
	"hash": {
		`printf 'abc' | agent-utils hash -a md5           # 900150983cd24fb0d6963f7d28e17f72`,
		`agent-utils hash release.tar.gz`,
		`agent-utils hash -c "$expected" release.tar.gz   # exit 0 on match, 3 on mismatch`,
	},
	"hex": {
		`printf 'hi' | agent-utils hex          # 6869`,
		`printf '6869' | agent-utils hex -d     # hi`,
	},
	"img-resize": {
		`agent-utils img-resize -max 512 photo.jpg thumb.jpg`,
		`agent-utils img-resize -width 800 screenshot.png small.png`,
	},
	"jpeg2png": {
		`agent-utils jpeg2png photo.jpg photo.png`,
	},
	"json-fmt": {
		`printf '{"b":1,"a":[1,2]}' | agent-utils json-fmt -indent 4`,
		`agent-utils json-fmt -c data.json`,
		`agent-utils json-fmt -check data.json   # exit 0 if valid, 3 if not`,
	},
	"json2csv": {
		`printf '[{"name":"Jane","zip":"01234"}]' | agent-utils json2csv`,
		`agent-utils csv2json data.csv | agent-utils json2csv`,
	},
	"json2toml": {
		`printf '{"name":"demo","port":8080}' | agent-utils json2toml`,
	},
	"json2yaml": {
		`printf '{"name":"demo","tags":["a","b"]}' | agent-utils json2yaml`,
	},
	"jwt-decode": {
		`printf '%s' "$TOKEN" | agent-utils jwt-decode | jq .payload.exp`,
	},
	"lorem": {
		`agent-utils lorem -w 5   # Lorem ipsum dolor sit amet,`,
		`agent-utils lorem -p 3`,
	},
	"md-table": {
		`printf '[{"name":"Jane","age":30}]' | agent-utils md-table`,
		`agent-utils md-table -csv report.csv`,
	},
	"password": {
		`agent-utils password`,
		`agent-utils password -l 32 -n 5 -no-symbols`,
	},
	"png2jpeg": {
		`agent-utils png2jpeg -quality 95 in.png out.jpg`,
	},
	"qr": {
		`agent-utils qr -o qr.png 'https://example.com'`,
		`agent-utils qr -d qr.png   # https://example.com`,
	},
	"semver": {
		`printf '1.2.0\n1.10.0\n' | agent-utils semver -r | head -1   # 1.10.0`,
		`agent-utils semver -compare 1.2.3 1.10.0                    # -1`,
		`agent-utils semver -check '>=1.2.0 <2.0.0' v1.5.3           # true`,
	},
	"slugify": {
		`printf 'Hello, World!' | agent-utils slugify   # hello-world`,
	},
	"strip-ansi": {
		`printf '\033[31mred\033[0m\n' | agent-utils strip-ansi   # red`,
		`agent-utils strip-ansi ci.log clean.log`,
	},
	"time": {
		`agent-utils time 1755459000                    # 2025-08-17T19:30:00Z`,
		`agent-utils time -z Asia/Tokyo 1755459000      # 2025-08-18T04:30:00+09:00`,
		`agent-utils time -f unix 2025-08-17T19:30:00Z  # 1755459000`,
	},
	"tiff2png": {
		`agent-utils tiff2png scan.tiff scan.png`,
	},
	"toml2json": {
		`agent-utils toml2json pyproject.toml | jq -r .project.name`,
	},
	"unicode": {
		`printf 'e\xcc\x81' | agent-utils unicode | jq -r '.[].codepoint'   # U+0065, U+0301`,
		`printf 'e\xcc\x81' | agent-utils unicode -nfc | agent-utils unicode | jq -r '.[].codepoint'   # U+00E9`,
		`agent-utils unicode -check suspicious.txt | jq .findings`,
	},
	"url": {
		`printf 'a b&c' | agent-utils url      # a+b%26c`,
		`echo 'a+b%26c' | agent-utils url -d   # a b&c`,
	},
	"urlparse": {
		`agent-utils urlparse 'https://example.com:8443/x?q=1' | jq -r .port   # 8443`,
	},
	"uuid": {
		`agent-utils uuid`,
		`agent-utils uuid -n 5`,
	},
	"version": {
		`agent-utils version`,
	},
	"video": {
		`agent-utils video 'https://www.youtube.com/watch?v=...'`,
		`agent-utils video -audio -o ~/Music 'https://www.youtube.com/watch?v=...'`,
	},
	"webp2png": {
		`agent-utils webp2png photo.webp photo.png`,
	},
	"xml2json": {
		`printf '<a id="7"><b>x</b></a>' | agent-utils xml2json | jq -r '.a["@id"]'   # 7`,
	},
	"yaml2json": {
		`agent-utils yaml2json config.yaml | jq .service.name`,
	},
}
