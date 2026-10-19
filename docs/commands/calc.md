# `calc`

Evaluate an arithmetic expression exactly: the sums, products, and powers a model gets subtly wrong. Every number is an exact rational (Go's `math/big.Rat`), never a float, so `0.1 + 0.2` is `0.3` and `2^64` is every digit of 18446744073709551616. The expression is a literal argument or comes from `stdin`.

## Usage

```sh
agent-utils calc [-scale n] [-json] [expression]
```

## Flags

| Flag         | Description |
| ------------ | ----------- |
| `-scale n`   | Round the result to exactly `n` decimal places (0 to 1000), padded with zeros, as for money. Without it, the result is exact when its decimal terminates and rounded to 20 places when it does not. |
| `-json`      | Print an object with the decimal `result`, the exact `fraction`, and whether the result is `exact`. |

The expression is one argument, so quote it: `agent-utils calc '(1 + 2) * 3'`. Unquoted, the shell splits it into several arguments (a usage error, exit 2), expands `*` as a glob, and rejects parentheses. With no argument, the expression is read from `stdin`, minus one trailing newline. An expression may start with a minus sign (`agent-utils calc '-2 * 3'`); `--` also ends the flags as usual.

### Grammar

| Syntax | Meaning |
| ------ | ------- |
| `12`, `0.5`, `.5`, `1.5e3`, `2E-4` | Decimal numbers, read exactly. No digit grouping (`1,000`), hex, or trailing decimal point (`5.`). |
| `( )` | Grouping. |
| `^` | Power, right-associative: `2^3^2` is `2^9`. The exponent must be an integer (negative allowed: `2^-2` is `0.25`); `0^0` is `1`. |
| `+x`, `-x` | Sign. A power binds tighter than a sign on its left, so `-2^2` is `-4`, as in mathematics. |
| `*`, `/`, `%` | Multiply, divide, and remainder, left-associative. |
| `+`, `-` | Add and subtract, left-associative. |

Operators are listed from tightest to loosest binding. Whitespace (spaces, tabs, newlines) is allowed anywhere between tokens. There are no functions, constants, or variables: `sqrt(2)` and `pi` fail with a message saying so, because their results are irrational and cannot be exact. Typographic operators (`−`, `×`, `÷`) and Python's `**` are rejected with a hint naming the ASCII operator.

`%` is the floored remainder `x - y * floor(x / y)`, defined for any rationals (`7.5 % 2` is `1.5`). Its sign follows the divisor, as in Python and mathematics: `-7 % 3` is `2` and `7 % -3` is `-2`. This differs from C, Go, and JavaScript, whose `%` follows the dividend.

### Output

The result prints in plain decimal notation, never with an exponent. Without `-scale`, an integer prints as an integer and a terminating decimal prints exactly, with no trailing zeros, however many places it needs (`2^-30` prints all 30). A non-terminating decimal (`1/3`) is rounded to exactly 20 places. With `-scale n`, every result is rounded to exactly `n` places and padded with zeros: `-scale 2` prints `5` as `5.00`.

Rounding is to the nearest value, with halves rounded away from zero, the rounding taught in school: `0.125` at `-scale 2` is `0.13`, and `-0.125` is `-0.13`. A result that rounds to zero prints without a sign. Use `-json` to see whether rounding happened:

```json
{
  "result": "0.33333333333333333333",
  "fraction": "1/3",
  "exact": false
}
```

`result` is the decimal exactly as calc would print it, `fraction` is the exact value in lowest terms (always `p/q`, so `5` is `5/1`), and `exact` is `true` when `result` equals that value. `result` and `fraction` are strings, not JSON numbers, so no consumer can lose precision.

### Errors and limits

These exit `1`: an empty expression, a syntax error, an unknown name, division or modulo by zero, a fractional exponent, and a result past the size limit. Each error but the first names the column where it occurred, counted in characters from 1. A syntax error is reported in preference to an evaluation error elsewhere in the expression. A bad `-scale` value or more than one expression argument is a usage error and exits `2`.

To keep `10^10^10` from exhausting memory, every number, including intermediate results, is limited to a numerator and denominator of 65536 bits (about 19,729 decimal digits). Exceeding it fails at once with an error naming the operator that would have; `2^65535` is within the limit and `2^65536` is not. Parentheses, signs, and chained powers nest at most 1000 deep.

## Examples

```sh
agent-utils calc '0.1 + 0.2'                       # 0.3
agent-utils calc '2^64'                            # 18446744073709551616
agent-utils calc '1/3'                             # 0.33333333333333333333
agent-utils calc '-2^2'                            # -4
agent-utils calc -scale 2 '19.99 * 3 * 1.0825'     # 64.92
agent-utils calc -json '1/3' | jq -r .fraction     # 1/3
printf '(1 + 2) * 3\n' | agent-utils calc          # 9
agent-utils calc '1/0'                             # error: division by zero at column 2 (exit 1)
```

---

[All commands](../../README.md#commands)
