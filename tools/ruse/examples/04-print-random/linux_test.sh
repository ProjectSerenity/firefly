#!/bin/bash

package="04-print-random"
output="$(./tools/ruse/examples/$package/hello-world_/hello-world)"
code="$?"
even="even"
[[ "$(($code % 2))" = "0" ]] || even="odd"
want="$(printf "0x%02x is %s." $code $even)"
[[ "$output" = "$want" ]] || (printf '%s: Output mismatch: got "%s", want "%s"\n' "$package" "$output" "$want" && exit 1)
