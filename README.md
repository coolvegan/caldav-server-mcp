# caldav-server

Minimal CalDAV-Server in Go – ein Nutzer, ein Kalender, Dateisystem-Backend.

## Quickstart

```bash
go build -o caldav-server .
CALDAV_USER=marco CALDAV_PASS=passwort ./caldav-server
```

Server läuft auf `:8080`. Kalender-URL: `http://localhost:8080/calendar/`

## Modi

| Befehl | Typ | Port | Transport |
|--------|-----|------|-----------|
| `./caldav-server` | HTTP CalDAV | 8080 | REST |
| `./caldav-server mcp` | MCP Tools | STDIO | JSON-RPC |
| `./caldav-server mcp-sse` | MCP Tools | 8081 | SSE (HTTP) |

## HTTP Endpunkte

| Methode | Pfad | Funktion |
|---------|------|----------|
| `OPTIONS` | `/calendar/` | Discovery (DAV-Header) |
| `PROPFIND` | `/calendar/` | Events auflisten (XML) |
| `GET` | `/calendar/{uid}.ics` | Ein Event lesen |
| `PUT` | `/calendar/` | Event erstellen/aktualisieren |
| `DELETE` | `/calendar/{uid}.ics` | Event löschen |
| `REPORT` | `/calendar/` | calendar-query + calendar-multiget |

Auth: HTTP Basic (`CALDAV_USER` / `CALDAV_PASS`)

## MCP Tools

| Tool | Parameter |
|------|-----------|
| `list_events` | – |
| `create_event` | `summary`, `dtstart`, `dtend` |
| `delete_event` | `uid` |

## Clients

- **Thunderbird**: Neuer Kalender → Im Netzwerk → CalDAV → `http://host:8080/calendar/`
- **DAVx⁵** (Android): CalDAV-Konto → URL + Login
- **opencode**: Lokaler MCP-Server → `./caldav-server mcp`
- **VS Code**: MCP-Server in `mcp.json`

## Env-Variablen

| Variable | Beschreibung |
|----------|-------------|
| `CALDAV_USER` | Benutzername für Basic Auth |
| `CALDAV_PASS` | Passwort für Basic Auth |
| `CALDAV_DIR` | Kalender-Verzeichnis (default: `./calendar`) |

## Dateistruktur

```
calendar/
├── event-abc.ics   # iCalendar-Dateien (RFC 5545)
├── event-def.ics
└── sync.json       # Sync-Token + Versions-Map
```

## Sync

Inkrementelle Synchronisation per REPORT mit `<sync-token>`. Zähler wird bei jedem `PUT` erhöht, Änderungen in `sync.json` persistiert.
