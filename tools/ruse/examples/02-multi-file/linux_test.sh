#!/bin/bash

set -e

package="02-multi-file"
output="$(./tools/ruse/examples/$package/hello-world_/hello-world)"
want="Hello, World!"
[[ "$output" = "$want" ]] || (printf '%s: Output mismatch: got "%s", want "%s"\n' "$package" "$output" "$want" && exit 1)
