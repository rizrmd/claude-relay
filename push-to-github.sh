#\!/bin/bash

# GitHub repository setup script
# Update these variables with your information
GITHUB_USERNAME="yourusername"
REPO_NAME="claude-relay"

echo "======================================"
echo "GitHub Repository Setup for Claude Relay"
echo "======================================"
echo ""
echo "Prerequisites:"
echo "1. You must have GitHub CLI (gh) installed"
echo "2. You must be authenticated with GitHub"
echo ""
echo "To install GitHub CLI:"
echo "  brew install gh"
echo ""
echo "To authenticate:"
echo "  gh auth login"
echo ""
echo "======================================"
echo ""

# Initialize git if not already
if [ \! -d .git ]; then
    echo "Initializing git repository..."
    git init
fi

# Add all files
echo "Adding files to git..."
git add .

# Create initial commit
echo "Creating initial commit..."
git commit -m "Initial commit: Claude Relay Go Library

- WebSocket relay server for Claude Code CLI
- Isolated, portable Bun and Claude installation
- Multiple authentication methods (interactive and programmatic)
- Support for multiple concurrent instances
- Comprehensive examples and documentation"

# Create GitHub repository using gh CLI
echo ""
echo "Creating GitHub repository..."
echo "Run this command after updating the username:"
echo ""
echo "gh repo create $REPO_NAME --public --description 'Go library for creating WebSocket relay servers for Claude Code CLI with isolated installations' --source=. --push"
echo ""
echo "Or manually:"
echo "1. Go to https://github.com/new"
echo "2. Create a new repository named: $REPO_NAME"
echo "3. Then run:"
echo "   git remote add origin https://github.com/$GITHUB_USERNAME/$REPO_NAME.git"
echo "   git branch -M main"
echo "   git push -u origin main"
echo ""
echo "======================================"
echo "After pushing, update import paths:"
echo "1. Replace 'claude-relay' with 'github.com/$GITHUB_USERNAME/$REPO_NAME' in examples"
echo "2. Update go.mod if needed"
echo "3. Tag a release: git tag v1.0.0 && git push origin v1.0.0"
