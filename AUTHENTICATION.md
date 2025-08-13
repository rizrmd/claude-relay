# Claude Relay - Authentication Guide

## Important: No API Keys!

**This library uses Claude Code CLI authentication, NOT Claude API keys.**

Claude Code CLI uses browser-based authentication through your Anthropic account. There are no API keys like `sk-ant-...` involved.

## How Claude Code CLI Authentication Works

1. User runs Claude Code CLI
2. Types `/login` command
3. Browser opens for Anthropic account login
4. CLI saves authentication token locally
5. Token stored in `.config/claude/auth.json`

## Authentication Methods

### 1. Interactive Authentication (Development)

For local development with terminal and browser access:

```go
relay, _ := clauderelay.New(clauderelay.Options{
    Port:    "8081",
    BaseDir: "./claude",
})

// Check if authenticated
if authenticated, _ := relay.IsAuthenticated(); !authenticated {
    // Run interactive login
    relay.Authenticate()
    // User will:
    // 1. See Claude interface
    // 2. Choose theme (press 1)
    // 3. Type /login
    // 4. Complete browser auth
}
```

### 2. Pre-configured Authentication (Production)

For servers, Docker, CI/CD without browser access:

#### Step 1: Authenticate on Development Machine

```bash
# Run the relay locally
./claude-relay -port 8081

# Complete the authentication process
# Files will be saved in .claude-home/.config/claude/
```

#### Step 2: Backup Authentication Files

```bash
# Copy the auth directory
cp -r .claude-home/.config/claude/ /backup/claude-auth/

# Or tar it for transfer
tar -czf claude-auth.tar.gz .claude-home/.config/claude/
```

#### Step 3: Use in Production

```go
relay, _ := clauderelay.New(clauderelay.Options{
    Port:             "8081",
    BaseDir:          "./claude",
    PreAuthDirectory: "/backup/claude-auth/", // Point to backed up auth
})
```

## Docker Deployment

### Creating Docker Image with Auth

```dockerfile
FROM alpine:latest

# Copy pre-authenticated config into image
COPY ./claude-auth /auth/claude

# Set environment
ENV CLAUDE_AUTH_DIR=/auth/claude

CMD ["./claude-relay"]
```

### Using Volume Mount

```bash
# Run with auth mounted as volume
docker run -v /local/claude-auth:/auth/claude \
    -e CLAUDE_AUTH_DIR=/auth/claude \
    -p 8081:8081 \
    your-image
```

## Kubernetes Deployment

### Using ConfigMap

```yaml
# First, create configmap from auth files
kubectl create configmap claude-auth \
    --from-file=auth.json=.claude-home/.config/claude/auth.json

# Deployment
apiVersion: apps/v1
kind: Deployment
metadata:
  name: claude-relay
spec:
  template:
    spec:
      containers:
      - name: claude-relay
        image: your-image
        env:
        - name: CLAUDE_AUTH_DIR
          value: /auth/claude
        volumeMounts:
        - name: auth
          mountPath: /auth/claude
      volumes:
      - name: auth
        configMap:
          name: claude-auth
```

### Using Secrets (Recommended)

```bash
# Create secret
kubectl create secret generic claude-auth \
    --from-file=auth.json=.claude-home/.config/claude/auth.json

# Use in deployment (similar to ConfigMap but with secret volume)
```

## CI/CD Pipeline

### GitHub Actions

```yaml
- name: Setup Claude Auth
  run: |
    mkdir -p .claude-home/.config/claude
    echo "${{ secrets.CLAUDE_AUTH_JSON }}" > .claude-home/.config/claude/auth.json

- name: Run with Claude
  run: |
    ./claude-relay -port 8081
```

### GitLab CI

```yaml
before_script:
  - mkdir -p .claude-home/.config/claude
  - echo "$CLAUDE_AUTH_JSON" > .claude-home/.config/claude/auth.json
```

## Authentication File Structure

The authentication is stored in a simple JSON file:

```
.claude-home/
└── .config/
    └── claude/
        └── auth.json  # Contains session token (NOT an API key)
```

## Checking Authentication

```go
// Simple check
authenticated, _ := relay.IsAuthenticated()

// Detailed status
authenticated, message, _ := relay.GetAuthStatus()
// Possible messages:
// - "Authenticated"
// - "No authentication file found"
// - "Authentication file is empty"
// - "Authentication file appears invalid"

// Get auth config path
path := relay.GetAuthConfigPath()
// Use this to backup auth files
```

## Transferring Authentication

### Manual Process

1. **On Machine A (with browser):**
```bash
# Authenticate
./claude-relay -port 8081
# Complete /login process

# Find auth files
ls -la .claude-home/.config/claude/

# Create backup
tar -czf claude-auth-backup.tar.gz .claude-home/.config/claude/
```

2. **On Machine B (production):**
```bash
# Extract backup
tar -xzf claude-auth-backup.tar.gz

# Use in code
relay.CopyAuthFrom(".claude-home/.config/claude/")
```

### Programmatic Transfer

```go
// On authenticated machine
sourceRelay, _ := clauderelay.New(clauderelay.Options{
    BaseDir: "./authenticated-instance",
})
authPath := sourceRelay.GetAuthConfigPath()

// Copy files from authPath to your deployment

// On production machine
prodRelay, _ := clauderelay.New(clauderelay.Options{
    BaseDir: "./production-instance",
})
prodRelay.CopyAuthFrom("/path/to/copied/auth")
```

## Security Considerations

1. **Protect Auth Files**: The `auth.json` file contains session tokens. Treat it like a password.

2. **Use Secrets Management**: In production, use proper secrets management:
   - Kubernetes Secrets
   - Docker Secrets
   - AWS Secrets Manager
   - HashiCorp Vault

3. **Limit Access**: Set file permissions to 0600 for auth files.

4. **Rotate Regularly**: Re-authenticate periodically for security.

5. **Don't Commit**: Never commit auth.json to version control. Add to .gitignore:
   ```
   .claude-home/
   **/auth.json
   ```

## Troubleshooting

### "Claude is not authenticated"
- No auth.json file exists
- Run `relay.Authenticate()` on a machine with browser access

### "Authentication file appears invalid"
- The auth.json file is corrupted or incomplete
- Re-authenticate with `relay.Authenticate()`

### Authentication works locally but not in Docker
- Check if auth files are properly copied/mounted
- Verify CLAUDE_AUTH_DIR environment variable
- Check file permissions in container

### Can't complete /login in headless environment
- You cannot authenticate in headless environments
- Must authenticate on a machine with browser access first
- Then copy auth files to headless environment

## FAQ

**Q: Can I use Claude API keys?**
A: No. This library uses Claude Code CLI, which has its own auth system.

**Q: How long does authentication last?**
A: Sessions typically last several weeks but may expire. Re-authenticate when needed.

**Q: Can I authenticate programmatically without a browser?**
A: No. Initial authentication requires browser access. After that, you can copy auth files.

**Q: Is the auth token the same as an API key?**
A: No. It's a session token specific to Claude Code CLI.

**Q: Can multiple instances share the same auth?**
A: Yes, you can copy auth files to multiple instances.