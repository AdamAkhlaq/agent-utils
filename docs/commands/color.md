# `color`

Convert a color between hex, rgb, and hsl: exact conversion math an agent would otherwise approximate. The color is a literal argument or comes from stdin.

## Usage

```sh
agent-utils color -to <hex|rgb|hsl> [color]
agent-utils color -json [color]
```

## Flags

| Flag         | Description                                                     |
| ------------ | --------------------------------------------------------------- |
| `-to <form>` | Output form: `hex`, `rgb`, or `hsl`. Required unless `-json`.   |
| `-json`      | Print all three forms as one JSON object (mutually exclusive with `-to`). |

Accepted input forms, case-insensitive: `#rgb` or `#rrggbb` hex (leading `#` optional), `rgb(255, 136, 0)` or the bare triple `255,136,0` with integer components 0-255, and `hsl(30, 100%, 50%)` with an integer hue 0-360 (360 means 0) and integer percentages. Alpha forms (`#rrggbbaa`, `rgba()`, `hsla()`) are rejected with a specific error, never silently stripped. There is no default output form: requiring `-to` keeps the output independent of the input's form, so scripts never guess.

Output is canonical: lowercase `#rrggbb`, `rgb(r, g, b)`, and `hsl(h, s%, l%)` with the hue in 0-359 and integer percentages. The canonical space is 8-bit RGB (hsl input converts to RGB first). Both directions use the CSS Color formulas computed exactly, in integers, and round each result (hsl components, and rgb channels from hsl input) to the nearest integer, halves away from zero, so a component that is exactly a half always rounds up: `#001801` is `hsl(123, 100%, 5%)` (its hue is 122.5), and `hsl(0, 100%, 5%)` is `rgb(26, 0, 0)` (its red channel is 25.5). The same formulas in floating point come out one off for about 1 hex color in 300 for that reason. That rounding makes hex to hsl lossy for near neighbors: 16.7M RGB values map onto roughly 2.4M rounded HSL triples, so for example `#ff8801` reads back from its hsl form as `#ff8800`. Primaries and greys round-trip exactly. An hsl value can come back changed too, because hsl input is rounded to 8-bit RGB first: `hsl(7, 30%, 40%)` is `#854f47`, which reads back as `hsl(8, 30%, 40%)`.

## Examples

```sh
agent-utils color -to hsl '#ff8800'            # hsl(32, 100%, 50%)
agent-utils color -to hex 'rgb(255, 136, 0)'   # #ff8800
agent-utils color -to rgb 'hsl(32, 100%, 50%)' # rgb(255, 136, 0)
printf '#663399' | agent-utils color -to hsl   # hsl(270, 50%, 40%)
agent-utils color -json '#ff8800' | jq .hsl.h  # 32
```

---

[All commands](../../README.md#commands)
