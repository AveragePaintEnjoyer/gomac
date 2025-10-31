# gomac

**gomac** is a lightweight SNMP-based switch monitoring tool written in Go.  
It allows you to monitor switch ports and MAC addresses via a web interface.

The main purpose of go-mac is to help track and document complex or poorly documented enterprise networks, showing which ports are active and what devices are connected to them.

## Features

- Polls switches using SNMP (supports Generic and UniFi systems)
- Tracks port status (`UP` / `DOWN`) and status change count
- Tracks MAC addresses on each port, including VLANs (if supported)
- Web dashboard displaying switches and ports in a "switch front panel" style
- Clickable ports to view detailed status and MAC tables
- Admin interface to add, remove, or edit switches
- MAC search page for finding which switch/port a MAC address is on
- Configurable via `.env` for:
  - Web server port
  - Polling interval
  - Database location

## Installation

1. Add user

```bash
sudo useradd -r -m -d /home/gomac gomac
```

2. Install necessary tools 

**Make sure to install current supported go version from golang website directly on to your system.**

```bash
sudo apt update
sudo apt install build-essential libsqlite3-dev
```

3. Clone repo

```bash
su gomac
cd /home/gomac/gomac

git clone https://github.com/AveragePaintEnjoyer/gomac
cd gomac
```

4. Compile

When compiling, use this flag:

```
go mod tidy

export CGO_ENABLED=1
go build -o gomac ./cmd/server
```

5. Set up .env file (example provided in `.config/.env.example`):

```bash
WEB_HOST=0.0.0.0
WEB_PORT=8080
POLL_INTERVAL=600
DB_PATH=/home/gomac/gomac/gomac.db
```

6. Run the server manually:

```bash
./gomac
```

The web interface will be available at [http://localhost:8080](http://localhost:8080).

7. Add systemd service (example provided in `.config/gomac.service`)

```bash
sudo nano /etc/systemd/system/gomac.service
```

8. Start

```bash
sudo systemctl daemon-reload
sudo systemctl enable gomac
sudo systemctl start gomac
sudo journalctl -u gomac -f 
```

## Usage

Dashboard: View all switches, port status, and MAC addresses.

![](.assets/02-dashboard-ext.png)

MAC Search: Search for a MAC address to see which switch and port it’s on.

![](.assets/03-mac.png)

Test: Test SNMP connectivity before adding switches to monitoring.

![](.assets/04-test.png)

Admin: Add or remove switches and configure port counts.

![](.assets/05-management.png)

## Dependencies

- Go 1.20+
- Fiber web framework
- GORM for SQLite
- GoSNMP for SNMP polling
- Tailwind CSS for UI styling (embedded in HTML templates)