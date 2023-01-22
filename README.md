## `gh changelog`

`gh changelog` is a GitHub CLI extension to generate a changelog from the PRs
of a milestone.

## Usage

```bash
$ gh changelog --help

  usage:
    gh changelog              Generate the changelog

  [options]
    -m, --milestone           Set a specific milestone
    -u, --unreleased          Set a version number for the unreleased changes
    -h, --help                Display the help information
```

## Installation

Make sure you have `gh` and `git` installed.

Then run:

```bash
$ gh extension install leofeyer/gh-changelog
```
