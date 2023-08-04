
# v1.3.6:

### Minor Updates
- [Feature #431](https://github.com/google/osv-scanner/pull/431)
  Update GoVulnCheck integration. 
- [Feature #439](https://github.com/google/osv-scanner/pull/439)
  Create `models.PURLToPackage()`, and deprecate `osvscanner.PURLToPackage()`.

### Fixes
- [Feature #439](https://github.com/google/osv-scanner/pull/439)
  Fix `PURLToPackage` not returning the full namespace of packages in ecosystems
  that use them (e.g. golang).


# v1.3.5:

### Features
- [Feature #409](https://github.com/google/osv-scanner/pull/409) 
  Adds an additional column to the table output which shows the severity if available.

### API Features

- [Feature #424](https://github.com/google/osv-scanner/pull/424)
- [Feature #417](https://github.com/google/osv-scanner/pull/417)
- [Feature #417](https://github.com/google/osv-scanner/pull/417)
  - Update the models package to better reflect the osv schema, including:
    - Add the withdrawn field
    - Improve timestamp serialization
    - Add related field
    - Add additional ecosystem constants
    - Add new reference types
    - Add YAML tags


# v1.3.4:

### Minor Updates

- [Feature #390](https://github.com/google/osv-scanner/pull/390) Add an
  user agent to OSV API requests.

# v1.3.3:

### Fixes

-   [Bug #369](https://github.com/google/osv-scanner/issues/369) Fix
    requirements.txt misparsing lines that contain `--hash`.
-   [Bug #237](https://github.com/google/osv-scanner/issues/237) Clarify when no
    vulnerabilities are found.
-   [Bug #354](https://github.com/google/osv-scanner/issues/354) Fix cycle in
    requirements.txt causing infinite recursion.
-   [Bug #367](https://github.com/google/osv-scanner/issues/367) Fix panic when
    parsing empty lockfile.

### API Features

-   [Feature #357](https://github.com/google/osv-scanner/pull/357) Update
    `pkg/osv` to allow overriding the http client / transport

# v1.3.2:

### Fixes

-   [Bug #341](https://github.com/google/osv-scanner/pull/341) Make the reporter
    public to allow calling DoScan with non nil reporters.
-   [Bug #335](https://github.com/google/osv-scanner/issues/335) Improve SBOM
    parsing and relaxing name requirements when explicitly scanning with
    `--sbom`.
-   [Bug #333](https://github.com/google/osv-scanner/issues/333) Improve
    scanning speed for regex heavy lockfiles by caching regex compilation.
-   [Bug #349](https://github.com/google/osv-scanner/pull/349) Improve SBOM
    documentation and error messages.

# v1.3.1:

### Fixes

-   [Bug #319](https://github.com/google/osv-scanner/issues/319) Fix
    segmentation fault when parsing CycloneDX without dependencies.

# v1.3.0:

### Major Features:

-   [Feature #198](https://github.com/google/osv-scanner/pull/198) GoVulnCheck
    integration! Try it out when scanning go code by adding the
    `--experimental-call-analysis` flag.
-   [Feature #260](https://github.com/google/osv-scanner/pull/198) Support `-r`
    flag in `requirements.txt` files.
-   [Feature #300](https://github.com/google/osv-scanner/pull/300) Make
    `IgnoredVulns` also ignore aliases.
-   [Feature #304](https://github.com/google/osv-scanner/pull/304) OSV-Scanner
    now runs faster when there's multiple vulnerabilities.

### Fixes

-   [Bug #249](https://github.com/google/osv-scanner/issues/249) Support yarn
    locks with quoted properties.
-   [Bug #232](https://github.com/google/osv-scanner/issues/232) Parse nested
    CycloneDX components correctly.
-   [Bug #257](https://github.com/google/osv-scanner/issues/257) More specific
    cyclone dx parsing.
-   [Bug #256](https://github.com/google/osv-scanner/issues/256) Avoid panic
    when parsing `file:` dependencies in `pnpm` lockfiles.
-   [Bug #261](https://github.com/google/osv-scanner/issues/261) Deduplicate
    packages that appear multiple times in `Pipenv.lock` files.
-   [Bug #267](https://github.com/google/osv-scanner/issues/267) Properly handle
    comparing zero versions in Maven.
-   [Bug #279](https://github.com/google/osv-scanner/issues/279) Trim leading
    zeros off when comparing numerical components in Maven versions.
-   [Bug #291](https://github.com/google/osv-scanner/issues/291) Check if PURL
    is valid before adding it to queries.
-   [Bug #293](https://github.com/google/osv-scanner/issues/293) Avoid infinite
    loops parsing Maven poms with syntax errors
-   [Bug #295](https://github.com/google/osv-scanner/issues/295) Set version in
    the source code, this allows version to be displayed in most package
    managers.
-   [Bug #297](https://github.com/google/osv-scanner/issues/297) Support Pipenv
    develop packages without versions.

### API Features

-   [Feature #310](https://github.com/google/osv-scanner/pull/310) Improve the
    OSV models to allow for 3rd party use of the library.

# v1.2.0:

### Major Features:

-   [Feature #168](https://github.com/google/osv-scanner/pull/168) Support for
    scanning debian package status file, usually located in
    `/var/lib/dpkg/status`. Thanks @cmaritan
-   [Feature #94](https://github.com/google/osv-scanner/pull/94) Specify what
    parser should be used in `--lockfile`.
-   [Feature #158](https://github.com/google/osv-scanner/pull/158) Specify
    output format to use with the `--format` flag.
-   [Feature #165](https://github.com/google/osv-scanner/pull/165) Respect
    `.gitignore` files by default when scanning.
-   [Feature #156](https://github.com/google/osv-scanner/pull/156) Support
    markdown table output format. Thanks @deftdawg
-   [Feature #59](https://github.com/google/osv-scanner/pull/59) Support
    `conan.lock` lockfiles and ecosystem Thanks @SSE4
-   Updated documentation! Check it out here:
    https://google.github.io/osv-scanner/

### Minor Updates:

-   [Feature #178](https://github.com/google/osv-scanner/pull/178) Support SPDX
    2.3.
-   [Feature #221](https://github.com/google/osv-scanner/pull/221) Support
    dependencyManagement section in Maven poms.
-   [Feature #167](https://github.com/google/osv-scanner/pull/167) Make
    osvscanner API library public.
-   [Feature #141](https://github.com/google/osv-scanner/pull/141) Retry OSV API
    calls to mitigate transient network issues. Thanks @davift
-   [Feature #220](https://github.com/google/osv-scanner/pull/220) Vulnerability
    output is ordered deterministically.
-   [Feature #179](https://github.com/google/osv-scanner/pull/179) Log number of
    packages scanned from SBOM.
-   General dependency updates

### Fixes

-   [Bug #161](https://github.com/google/osv-scanner/pull/161) Exit with non
    zero exit code when there is a general error.
-   [Bug #185](https://github.com/google/osv-scanner/pull/185) Properly omit
    Source from JSON output.

# v1.1.0:

This update adds support for NuGet ecosystem and various bug fixes by the
community.

-   [Feature #98](https://github.com/google/osv-scanner/pull/98): Support for
    NuGet ecosystem.
-   [Feature #71](https://github.com/google/osv-scanner/issues/71): Now supports
    Pipfile.lock scanning.
-   [Bug #85](https://github.com/google/osv-scanner/issues/85): Even better
    support for narrow terminals by shortening osv.dev URLs.
-   [Bug #105](https://github.com/google/osv-scanner/issues/105): Fix rare cases
    of too many open file handles.
-   [Bug #131](https://github.com/google/osv-scanner/pull/131): Fix table
    highlighting overflow.
-   [Bug #101](https://github.com/google/osv-scanner/issues/101): Now supports
    32 bit systems.

# v1.0.2

This is a minor patch release to mitigate human readable output issues on narrow
terminals (#85).

-   [Bug #85](https://github.com/google/osv-scanner/issues/85): Better support
    for narrow terminals.

# v1.0.1

Various bug fixes and improvements. Many thanks to the amazing contributions and
suggestions from the community!

-   Feature: ARM64 builds are now also available!
-   [Feature #46](https://github.com/google/osv-scanner/pull/46): Gradle
    lockfile support.
-   [Feature #50](https://github.com/google/osv-scanner/pull/46): Add version
    command.
-   [Bug #52](https://github.com/google/osv-scanner/issues/52): Fixes 0 exit
    code being wrongly emitted when vulnerabilities are present.
