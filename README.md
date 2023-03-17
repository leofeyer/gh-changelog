## `gh changelog`

`gh changelog` is a GitHub CLI extension to generate a changelog from the pull
requests of a milestone.

## Installation

Make sure you have `gh` and `git` installed. Then run:

```bash
$ gh extension install leofeyer/gh-changelog
```

## Usage

```bash
$ gh changelog 1.2
```

You can optionally specify a version number for unreleased changes:

```bash
$ gh changelog 1.2 1.2.6
```
