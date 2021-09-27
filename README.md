# Autocut

Automatically create new issues, or update existing ones by deduping on the
title.

With this tool, you can repeatedly "cut" the same issue over and over, knowing
it'll only be created once, updated if it's stale, and re-open if it was
recently closed. It was built to support system monitoring.

You can use it as a CLI, or as a Go library in your application.

## Build it

```
% go build ./cmd/autocut/...
```

## Use it

First, set `GITHUB_TOKEN` to your GitHub token.

```
% export GITHUB_TOKEN=abc123
```

Then make a new issue that something is wrong:

```
% ./autocut -owner allenai -repo aimichal -dur 10m -title "something is wrong" -details "bad habits"
Opened new issue https://github.com/allenai/aimichal/issues/27.
```

Any time in the next 10 minutes, repeating this will do nothing:

```
% ./autocut -owner allenai -repo aimichal -dur 10m -title "something is wrong" -details "bad habits"
Found recently updated issue, so did nothing: https://github.com/allenai/aimichal/issues/27
```

But after 10 minutes, running this will update the issue with a comment:

```
% ./autocut -owner allenai -repo aimichal -dur 10m -title "something is wrong" -details "such bad things are happening"
Found a stale issue, so commented on it: https://github.com/allenai/aimichal/issues/27
```

If this issue is closed, then running this will re-open the issue within 10
minutes. After 10 minutes, a new issue will be created.

Probably a good default duration to use is 24 hours (`-dur 24h`).
