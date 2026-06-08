# Installation

Planck is distributed as a single static binary with absolutely zero dependencies.

## Option 1: curl (Quick Install — macOS / Linux)

The fastest way to get started. Downloads and installs the binary in one command:

```bash
curl -sSL "https://github.com/mihirsn/planck/releases/latest/download/planck_$(uname -s | tr '[:upper:]' '[:lower:]')_$(uname -m | sed 's/x86_64/amd64/;s/aarch64/arm64/').tar.gz" | tar -xz
sudo mv planck /usr/local/bin/
```

## Option 2: .deb Package (Ubuntu / Debian)

The easiest way to install on Linux servers. The package includes a pre-configured `systemd` service file so you can run Planck as a background daemon automatically.

```bash
curl -OL https://github.com/mihirsn/planck/releases/latest/download/planck_linux_amd64.deb
sudo dpkg -i planck_linux_amd64.deb
```

> For arm64 (e.g., AWS Graviton), replace `amd64` with `arm64`.

## Option 3: .rpm Package (CentOS / Amazon Linux / RHEL)

```bash
curl -OL https://github.com/mihirsn/planck/releases/latest/download/planck_linux_amd64.rpm
sudo rpm -i planck_linux_amd64.rpm
```

> For arm64, replace `amd64` with `arm64`.

## Option 4: Go Install

If you have Go installed locally:

```bash
go install github.com/mihirsn/planck@latest
```

## Option 5: Build from Source

```bash
git clone https://github.com/mihirsn/planck.git
cd planck
make build
sudo mv planck /usr/local/bin/
```

## Verify Installation

```bash
planck --version
```
