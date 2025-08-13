# Claude Relay - Authentication Guide

## Overview

The Claude Relay library provides flexible authentication options to work in any environment - from interactive terminals to serverless functions.

## Authentication Methods

### 1. **SetAuthToken()** - Programmatic Authentication

**When to use:** Servers, containers, CI/CD, serverless, any headless environment

```go
// Direct method call
apiKey := os.Getenv("CLAUDE_API_KEY")
relay.SetAuthToken(apiKey)

// Or during initialization
relay, _ := clauderelay.New(clauderelay.Options{
    APIKey: os.Getenv("CLAUDE_API_KEY"),
})
```

**Characteristics:**
- ✅ No user interaction required
- ✅ Works without terminal
- ✅ Suitable for automation
- ✅ Can be changed at runtime

### 2. **Authenticate()** - Interactive Authentication  

**When to use:** CLI tools, local development, when user has terminal access

```go
if !relay.IsAuthenticated() {
    relay.Authenticate() // Launches Claude's interactive login
}
```

**User experience:**
1. Claude interface appears in terminal
2. User selects theme (press 1 for dark mode)
3. User types `/login`
4. Browser opens for authentication
5. Token is saved automatically

**Characteristics:**
- ❌ Requires terminal/TTY
- ❌ Needs user interaction
- ✅ User manages own credentials
- ✅ Familiar Claude experience

### 3. **AuthCallback** - Custom Authentication Flow

**When to use:** Web apps, mobile apps, custom UIs

```go
relay, _ := clauderelay.New(clauderelay.Options{
    AuthCallback: func(authURL string) (string, error) {
        // Your custom logic to get API key
        // - Show authURL in your UI
        // - Wait for user input
        // - Query database
        // - Call another service
        return apiKey, nil
    },
})
```

**Example - Web Application:**
```go
AuthCallback: func(authURL string) (string, error) {
    // Send URL to frontend
    websocket.Send(map[string]string{
        "type": "auth_required",
        "url": authURL,
    })
    
    // Wait for API key from frontend
    select {
    case apiKey := <-apiKeyChan:
        return apiKey, nil
    case <-time.After(5 * time.Minute):
        return "", fmt.Errorf("authentication timeout")
    }
}
```

## Checking Authentication Status

### Simple Check
```go
authenticated, err := relay.IsAuthenticated()
if !authenticated {
    // Handle authentication
}
```

### Detailed Status
```go
authenticated, message, err := relay.GetAuthStatus()
// Returns one of:
// - (true, "Authenticated", nil)
// - (false, "No authentication file found", nil)  
// - (false, "Authentication file is empty", nil)
// - (false, "Authentication file appears invalid", nil)
```

### Get Auth URL
```go
url, _ := relay.GetAuthURL()
// Returns: "https://console.anthropic.com/settings/keys"
```

## Use Case Examples

### Docker Container
```go
func main() {
    relay, err := clauderelay.New(clauderelay.Options{
        Port:   "8081",
        APIKey: os.Getenv("CLAUDE_API_KEY"), // From env or secret
    })
    if err != nil {
        log.Fatal(err)
    }
    
    // Already authenticated via APIKey option
    relay.Start()
}
```

### Kubernetes with Secrets
```go
// Read from mounted secret
apiKey, _ := os.ReadFile("/var/run/secrets/claude/api-key")

relay, _ := clauderelay.New(clauderelay.Options{
    Port: "8081",
})
relay.SetAuthToken(string(apiKey))
```

### AWS Lambda
```go
func handler(ctx context.Context) error {
    // Get from AWS Secrets Manager or Parameter Store
    apiKey := getSecretValue("claude-api-key")
    
    relay, _ := clauderelay.New(clauderelay.Options{
        APIKey: apiKey,
    })
    
    return relay.Start()
}
```

### Web Service with UI
```go
// See examples/webservice/main.go for full implementation
http.HandleFunc("/api/auth/set-key", func(w http.ResponseWriter, r *http.Request) {
    var req struct {
        APIKey string `json:"api_key"`
    }
    json.NewDecoder(r.Body).Decode(&req)
    
    err := relay.SetAuthToken(req.APIKey)
    if err != nil {
        http.Error(w, err.Error(), 500)
        return
    }
    
    fmt.Fprintf(w, `{"success": true}`)
})
```

### CLI Tool with Fallback
```go
relay, _ := clauderelay.New(clauderelay.Options{
    APIKey: os.Getenv("CLAUDE_API_KEY"), // Try env first
})

// If no env var, check if authenticated
if !relay.IsAuthenticated() {
    fmt.Println("No API key found. Starting interactive login...")
    relay.Authenticate() // Fall back to interactive
}
```

## Authentication Storage

Each relay instance stores authentication in:
```
BaseDir/
└── .claude-home/
    └── .config/
        └── claude/
            └── auth.json
```

**File format:**
```json
{"key":"sk-ant-api..."}
```

## Security Best Practices

### 1. Never hardcode API keys
```go
// ❌ Bad
APIKey: "sk-ant-api03-..."

// ✅ Good  
APIKey: os.Getenv("CLAUDE_API_KEY")
```

### 2. Use appropriate secret storage
- **Development**: Environment variables
- **Docker**: Build args or runtime secrets
- **Kubernetes**: Secret objects
- **Cloud**: AWS Secrets Manager, Azure Key Vault, GCP Secret Manager

### 3. Rotate keys regularly
```go
// Support key rotation without restart
go func() {
    for newKey := range keyRotationChannel {
        relay.SetAuthToken(newKey)
        log.Println("API key rotated")
    }
}()
```

### 4. Validate authentication
```go
relay.SetAuthToken(apiKey)

// Always verify it worked
authenticated, message, _ := relay.GetAuthStatus()
if !authenticated {
    log.Fatalf("Authentication failed: %s", message)
}
```

## Comparison Table

| Feature | SetAuthToken | Authenticate | AuthCallback |
|---------|--------------|--------------|--------------|
| No Terminal Needed | ✅ | ❌ | ✅ |
| No User Interaction | ✅ | ❌ | Depends |
| Works in Containers | ✅ | ❌ | ✅ |
| Works in Serverless | ✅ | ❌ | ✅ |
| Custom UI Possible | ❌ | ❌ | ✅ |
| User Manages Auth | ❌ | ✅ | Depends |

## Troubleshooting

### "Claude is not authenticated"
- Check if API key is set correctly
- Verify auth file exists: `BaseDir/.claude-home/.config/claude/auth.json`
- Check file permissions (should be 0600)

### "Invalid API key"
- Ensure key starts with `sk-ant-`
- Check for extra whitespace
- Verify key hasn't been revoked

### Authentication not persisting
- Check BaseDir is writable
- Ensure each instance uses unique BaseDir
- Verify no permission issues

## Migration Guide

### From Interactive to Non-Interactive

**Before (Terminal Required):**
```go
relay.Authenticate() // User must interact
```

**After (No Terminal Needed):**
```go
relay.SetAuthToken(os.Getenv("CLAUDE_API_KEY"))
```

### From Hardcoded to Dynamic

**Before:**
```go
// Hardcoded at initialization
relay, _ := clauderelay.New(clauderelay.Options{
    APIKey: "sk-ant-...",
})
```

**After:**
```go
// Can be set/changed anytime
relay, _ := clauderelay.New(clauderelay.Options{})
relay.SetAuthToken(getAPIKeyFromSource())