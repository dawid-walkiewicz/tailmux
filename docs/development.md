# Development

```sh
go test ./...          # unit + end-to-end, a few seconds
go test -race ./...    # what CI runs
```

## The lab

Tests run **three complete tailnets in-process** (`internal/lab`). Each one has
Tailscale's own test control server, a DERP relay, web servers and subnet
routers. The setup collides on purpose:

- every tailnet's `web` node gets the same IP;
- two tailnets route the same `192.0.2.0/24`;
- one tailnet serves `corp.internal` over split DNS.

To point the real binary, the menu bar app or `tailmux setup` at it:

```sh
TAILMUX_LAB=/tmp/lab.json go test ./internal/mux -run TestLab -timeout 0 &
go run ./cmd/tailmux up -config /tmp/lab.json
curl --socks5-hostname 127.0.0.1:21055 'http://web.bravo/?bytes=5000000' >/dev/null
TAILMUX_API=http://127.0.0.1:21056 macos/build/TailmuxBar.app/Contents/MacOS/TailmuxBar
```

## What CI checks

- **lint**: golangci-lint, `go mod tidy`, builds for linux and freebsd, govulncheck.
- **test**: race-enabled unit and end-to-end tests on Linux.
- **tun-e2e**: `TestSystem` as root on Linux. It creates a real TUN device,
  sets up routes and the system resolver, then runs plain
  `http.Get("http://web.bravo/")`.
- **service**: `tailmux service install` under systemd: the API, its token
  checks and TUN mode, then a clean uninstall.
- **desktop / desktop-uninstall**: builds the desktop `.rpm` on Fedora, runs
  its API tests and screenshots against a live daemon, and checks that
  removing the `.rpm` removes the service.
- **linux-packages / linux-distros**: builds the `tailmux-cli` packages with
  GoReleaser and installs each on its distro (Debian, Fedora, Alpine, Arch).
  On Fedora and Arch it also runs `make && make install`.

## Releasing

Push a `v*` tag. The release workflow then:

1. runs the tests;
2. publishes the Linux binaries and the `tailmux-cli` packages with GoReleaser;
3. attaches the desktop app as an `.rpm` built on Fedora.

## The README banner

`docs/banner.svg` is generated from `docs/panel-dark.png`, which is the menu
bar panel rendered against `scripts/demo-daemon.py`, a fake of the API with
tidy demo data. To rebuild both:

```sh
scripts/demo-daemon.py 21099 &
swift build -c release --disable-sandbox --package-path macos/TailmuxBar
TAILMUX_API=http://127.0.0.1:21099 macos/TailmuxBar/.build/release/TailmuxBar --snapshot docs/panel-dark.png --dark --scale 2
pip install fonttools brotli
scripts/banner.py --font path/to/Satoshi-Variable.woff2
```

Satoshi isn't in the repo because its license doesn't allow redistribution.
The script turns the text into outlines, so the SVG doesn't need the font.
