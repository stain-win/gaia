# Gaia Ansible Deployment

Ansible playbook for deploying Gaia Secrets Management daemon on Linux servers using pre-built binaries.

## Overview

This playbook automates the complete setup of Gaia on a Linux server, including:

- ✅ Downloading and installing the latest Gaia binary
- ✅ Creating dedicated system user and directories  
- ✅ Generating mTLS certificates
- ✅ Initializing the encrypted database
- ✅ Setting up systemd service
- ✅ Configuring firewall rules (optional)
- ✅ Setting up log rotation

## Requirements

### Control Node (your local machine)
- Ansible 2.9+
- SSH access to target servers

### Target Servers
- Ubuntu 20.04+, Debian 10+, RHEL/CentOS 8+, or Amazon Linux 2
- Python 3 (for Ansible modules)
- `sudo` access

## Quick Start

### 1. Configure Inventory

Edit `inventories/production/hosts.yml`:

```yaml
all:
  hosts:
    gaia-server:
      ansible_host: your-server-ip
      ansible_user: your-ssh-user
      ansible_become: true
```

### 2. Configure Variables (Required)

Edit `inventories/production/group_vars/all.yml`:

```yaml
# IMPORTANT: Add your SSH user to admin_users so you can run gaia commands
gaia_admin_users:
  - ubuntu          # Replace with your SSH username

gaia_version: "latest"           # or specific version like "v1.0.0"
gaia_listen_addr: "0.0.0.0:50051"
gaia_log_level: "info"
gaia_cert_expiry_days: 365
```

> ⚠️ **Important:** If you don't add your SSH user to `gaia_admin_users`, you won't be able to run `gaia` commands after deployment. You'll see "daemon not running" errors even though the daemon is running.

### 3. Run the Playbook

```bash
# Full installation
ansible-playbook -i inventories/production/hosts.yml site.yml

# With specific version
ansible-playbook -i inventories/production/hosts.yml site.yml \
  --extra-vars "gaia_version=v1.2.0"

# Dry-run (check mode)
ansible-playbook -i inventories/production/hosts.yml site.yml --check
```

### 4. Post-Installation

After installation, you need to:

1. **Log out and back in** (required for group membership to take effect):
   ```bash
   # Or use newgrp to activate immediately
   newgrp gaia
   ```

2. **Initialize the database** (if not done):
   ```bash
   # Run as gaia user for database initialization
   sudo -u gaia gaia init --config /etc/gaia/gaia-config.yaml
   # Enter your master passphrase when prompted
   ```

3. **Start and unlock the daemon**:
   ```bash
   sudo systemctl start gaia
   
   # Now you can use gaia commands directly (you're in gaia group)
   gaia daemon unlock --config /etc/gaia/gaia-config.yaml
   
   # Or use the TUI
   gaia
   ```

4. **Create client certificates** for your applications:
   ```bash
   # Create cert for an application
   gaia certs create-client --name myapp --config /etc/gaia/gaia-config.yaml
   
   # Copy to application server
   scp /etc/gaia/certs/myapp.crt user@app-server:/path/to/certs/
   scp /etc/gaia/certs/myapp.key user@app-server:/path/to/certs/
   scp /etc/gaia/certs/ca.crt user@app-server:/path/to/certs/
   ```

## Directory Structure

```
ansible/
├── README.md                      # This file
├── site.yml                       # Main playbook
├── inventories/
│   ├── production/
│   │   ├── hosts.yml              # Production inventory
│   │   └── group_vars/
│   │       └── all.yml            # Production variables
│   └── staging/
│       ├── hosts.yml              # Staging inventory
│       └── group_vars/
│           └── all.yml            # Staging variables
├── roles/
│   └── gaia/
│       ├── defaults/
│       │   └── main.yml           # Default variables
│       ├── tasks/
│       │   ├── main.yml           # Main task file
│       │   ├── install.yml        # Binary installation
│       │   ├── configure.yml      # Configuration
│       │   ├── certificates.yml   # Certificate generation
│       │   └── service.yml        # Systemd setup
│       ├── handlers/
│       │   └── main.yml           # Handlers
│       ├── templates/
│       │   ├── gaia-config.yaml.j2  # Config template
│       │   └── gaia.service.j2      # Systemd template
│       └── vars/
│           └── main.yml           # Role variables
└── templates/
    └── logrotate.conf.j2          # Logrotate template
```

## Configuration Variables

| Variable | Default | Description |
|----------|---------|-------------|
| `gaia_version` | `"latest"` | Gaia version to install |
| `gaia_user` | `"gaia"` | System user for running daemon |
| `gaia_group` | `"gaia"` | System group |
| `gaia_admin_users` | `[]` | Users to add to gaia group (for CLI/TUI access) |
| `gaia_docker_acl_enabled` | `false` | Enable ACLs for Docker container access |
| `gaia_docker_group` | `"docker"` | Group for Docker container access |
| `gaia_install_dir` | `"/usr/local/bin"` | Binary installation path |
| `gaia_config_dir` | `"/etc/gaia"` | Configuration directory |
| `gaia_data_dir` | `"/var/lib/gaia"` | Database storage directory |
| `gaia_log_dir` | `"/var/log/gaia"` | Log files directory |
| `gaia_listen_addr` | `"0.0.0.0:50051"` | gRPC listen address |
| `gaia_log_level` | `"info"` | Log level (debug, info, warn, error) |
| `gaia_cert_expiry_days` | `365` | Certificate validity in days |
| `gaia_timeout` | `"10s"` | Operation timeout |
| `gaia_configure_firewall` | `false` | Whether to configure firewall |
| `gaia_firewall_allowed_ips` | `[]` | IPs allowed to access Gaia port |
| `gaia_audit_enabled` | `true` | Enable audit logging |
| `gaia_audit_log_path` | `"/var/log/gaia/audit.log"` | Audit log file path |

## Admin Users & Permissions

### Understanding the Permission Model

Gaia uses mTLS for authentication. Both the daemon and CLI/TUI clients need certificates to communicate:

| File | Mode | Purpose |
|------|------|---------|
| `ca.key` | `0600` | CA private key (critical - only gaia user) |
| `server.key` | `0600` | Server private key (daemon only) |
| `gaia.key` | `0640` | Admin client key (group-readable for CLI/TUI) |
| `*.crt` | `0644` | Certificates (public, readable by all) |

### Configuring Admin Access

To allow SSH users to run `gaia` commands (TUI, CLI), add them to `gaia_admin_users`:

```yaml
# inventories/production/group_vars/all.yml
gaia_admin_users:
  - ubuntu          # Your SSH user
  - deploy          # Deployment user
  - admin           # Additional admins
```

These users will be added to the `gaia` group and can:
- Run `gaia` TUI to manage secrets
- Run `gaia daemon status` to check daemon state
- Run `gaia certs create-client` to generate client certificates
- Unlock/lock the daemon

### Why This Matters

Without proper permissions, SSH users cannot read the admin certificates and will see:
```
Error: could not connect to daemon. Is it running?
```

Even though the daemon is running, the CLI can't authenticate because it can't read `gaia.key`.

### Manual Permission Fix

If you need to add an admin user after deployment:

```bash
# Add user to gaia group
sudo usermod -aG gaia your-username

# User must log out and back in for group membership to take effect
# Or run: newgrp gaia
```

## Docker Container Access (ACLs)

For Docker containers that need to read Gaia client certificates, we use **Default Access Control Lists (ACLs)**. This is the cleanest solution because:

- ✅ No need to change Gaia's umask
- ✅ No need to restart the Gaia service
- ✅ New client certs automatically inherit correct permissions
- ✅ Docker containers can read certs without running as root

### Enabling Docker ACLs

```yaml
# inventories/production/group_vars/all.yml
gaia_docker_acl_enabled: true
gaia_docker_group: "docker"
```

### How It Works

1. **Default ACL** is set on `/etc/gaia/certs/` directory
2. Any **new file** created in that directory automatically gets read permission for the `docker` group
3. Docker containers running as members of the `docker` group can read the certs

### Permission Model with ACLs

| File | Standard Mode | ACL (docker group) |
|------|---------------|-------------------|
| `ca.key` | `0600` | ❌ No access (protected) |
| `server.key` | `0600` | ❌ No access (protected) |
| `gaia.key` | `0640` | `r--` (read only) |
| `*.crt` | `0644` | `r--` (read only) |
| `<client>.key` | (new) | `r--` (inherited from default ACL) |

### Using Certs in Docker Compose

```yaml
services:
  myapp:
    image: myapp:latest
    volumes:
      - /etc/gaia/certs/myapp.crt:/app/certs/client.crt:ro
      - /etc/gaia/certs/myapp.key:/app/certs/client.key:ro
      - /etc/gaia/certs/ca.crt:/app/certs/ca.crt:ro
    environment:
      - GAIA_ADDRESS=host.docker.internal:50051
      - GAIA_CA_CERT=/app/certs/ca.crt
      - GAIA_CLIENT_CERT=/app/certs/client.crt
      - GAIA_CLIENT_KEY=/app/certs/client.key
```

### Verifying ACLs

```bash
# Check ACLs on the certs directory
getfacl /etc/gaia/certs/

# Expected output:
# # file: etc/gaia/certs/
# # owner: gaia
# # group: gaia
# user::rwx
# group::r-x
# group:docker:r-x          <- Docker group has rx
# mask::r-x
# other::---
# default:user::rwx
# default:group::r-x
# default:group:docker:r--  <- New files inherit read for docker
# default:mask::r-x
# default:other::---
```

### Troubleshooting Docker Access

**Container can't read certificates:**
```bash
# Check if docker group exists
getent group docker

# Check ACLs
getfacl /etc/gaia/certs/

# Re-apply ACLs
ansible-playbook -i inventories/production/hosts.yml site.yml --tags docker
```

**New certs don't have docker access:**
```bash
# Re-run the playbook to apply ACLs to new files
ansible-playbook -i inventories/production/hosts.yml site.yml --tags docker
```

## Tags

Use tags to run specific parts of the playbook:

```bash
# Only install binary
ansible-playbook -i inventories/production/hosts.yml site.yml --tags install

# Only configure
ansible-playbook -i inventories/production/hosts.yml site.yml --tags configure

# Only setup certificates
ansible-playbook -i inventories/production/hosts.yml site.yml --tags certificates

# Only setup systemd service
ansible-playbook -i inventories/production/hosts.yml site.yml --tags service
```

## Upgrading Gaia

To upgrade to a newer version:

```bash
# Upgrade to latest
ansible-playbook -i inventories/production/hosts.yml site.yml \
  --tags install,service \
  --extra-vars "gaia_version=latest"

# Upgrade to specific version  
ansible-playbook -i inventories/production/hosts.yml site.yml \
  --tags install,service \
  --extra-vars "gaia_version=v1.3.0"
```

The playbook will:
1. Download the new binary
2. Stop the service gracefully
3. Replace the binary
4. Restart the service

**Note:** After upgrade, you'll need to unlock the daemon again.

## Security Hardening

The playbook implements several security measures:

- **Dedicated user**: Gaia daemon runs as unprivileged `gaia` user
- **Admin group**: Users in `gaia` group can manage the daemon via CLI/TUI
- **Restrictive permissions**: 
  - CA & server private keys: `0600` (daemon only)
  - Admin client key: `0640` (group readable for CLI/TUI)
  - Config files: `0640` (group readable)
  - Database: `0600` (daemon only)
- **Systemd hardening**: PrivateTmp, ProtectSystem, NoNewPrivileges, MemoryDenyWriteExecute
- **Optional firewall**: Limit access to specific IPs

For additional security:

1. Enable full disk encryption (LUKS)
2. Use strong master passphrase (25+ chars)
3. Restrict SSH access
4. Set up regular backups
5. Monitor audit logs

## Troubleshooting

### "Could not connect to daemon" / "Is it running?"

This is usually a **permission issue**, not a daemon issue. The CLI can't read the admin certificates.

**Check 1: Are you in the gaia group?**
```bash
groups
# Should include 'gaia'
```

**Fix:** Add yourself to the gaia group:
```bash
sudo usermod -aG gaia $USER
# Log out and back in, or run:
newgrp gaia
```

**Check 2: Can you read the certificates?**
```bash
ls -la /etc/gaia/certs/
# gaia.key should be mode 0640 (group readable)
# ca.crt, gaia.crt should be mode 0644
```

**Fix:** Correct permissions:
```bash
sudo chmod 0640 /etc/gaia/certs/gaia.key
sudo chmod 0644 /etc/gaia/certs/gaia.crt /etc/gaia/certs/ca.crt
```

**Check 3: Is the daemon actually running?**
```bash
sudo systemctl status gaia
```

### Service won't start

```bash
# Check status
sudo systemctl status gaia

# Check logs
sudo journalctl -u gaia -e --no-pager

# Verify config
sudo -u gaia gaia --config /etc/gaia/gaia-config.yaml daemon status
```

### Certificate issues

```bash
# Check certificate permissions
ls -la /etc/gaia/certs/

# Expected permissions:
# ca.key     - 0600 (gaia:gaia)
# server.key - 0600 (gaia:gaia)  
# gaia.key   - 0640 (gaia:gaia) <- group readable!
# *.crt      - 0644 (gaia:gaia)

# Verify certificate validity
openssl x509 -in /etc/gaia/certs/server.crt -text -noout
```

### Database issues

```bash
# Check database file
ls -la /var/lib/gaia/gaia.db

# Re-initialize if needed (WARNING: destroys data!)
sudo -u gaia gaia init --config /etc/gaia/gaia-config.yaml
```

## Backup & Restore

### Backup

```bash
# Add to crontab
0 2 * * * /usr/local/bin/gaia-backup.sh
```

The playbook creates a backup script at `/usr/local/bin/gaia-backup.sh`.

### Restore

```bash
# Stop service
sudo systemctl stop gaia

# Restore database
sudo cp /backup/gaia/gaia-YYYYMMDD.db /var/lib/gaia/gaia.db
sudo chown gaia:gaia /var/lib/gaia/gaia.db

# Restore certs if needed
sudo tar -xzf /backup/gaia/certs-YYYYMMDD.tar.gz -C /etc/gaia/

# Start service
sudo systemctl start gaia
```

## License

MIT License - See [LICENSE](../../LICENSE) for details.
