# Gaia

A Go application that can run as a CLI or a long-running daemon.

## Usage

```
go run main.go [init|start|stop|restart|status]
```

- `init`    - Initialize Gaia
- `start`   - Start the Gaia daemon
- `stop`    - Stop the Gaia daemon
- `restart` - Restart the Gaia daemon
- `status`  - Show Gaia status

If no command is given, Gaia enters interactive mode.
module github.com/yourusername/gaia/apps/gaia

go 1.21

