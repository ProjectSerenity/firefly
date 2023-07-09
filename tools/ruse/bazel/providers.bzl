# Copyright 2023 The Firefly Authors.
#
# Use of this source code is governed by a BSD 3-clause
# license that can be found in the LICENSE file.

# Providers returned by the Ruse rules.

RusePackageInfo = provider(
    doc = "Contains information about a Ruse package",
    fields = {
        "deps": "A depset of info structs for this package's dependencies.",
        "info": """A struct containing information about this package.
        Has the following fields:
            package_path: The full package path of the package.
            rpkg: The .rpkg file containing the compiled package.
        """,
    },
)
