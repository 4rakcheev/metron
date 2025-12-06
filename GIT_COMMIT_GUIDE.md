# Git Commit Guide

This project is now ready for initial commit. Follow these steps:

## Pre-commit Checklist

✅ All tests passing (48/48)
✅ Code formatted (`make fmt`)
✅ Build successful (`make build`)
✅ Documentation complete
✅ .gitignore configured
✅ Sensitive files excluded

## Initialize Git Repository

```bash
cd /Users/tema/Pr/go/src/metron
git init
```

## Review Files to Commit

```bash
# Check what will be committed
git status

# Review .gitignore
cat .gitignore
```

## Important: Files NOT to Commit

The following are already in `.gitignore`:
- `config.json` - Contains API keys and secrets
- `*.db` files - Database files
- `bin/` - Compiled binaries
- `.idea/` - IDE settings

**Safe files in repo:**
- `config.example.json` - Template without secrets
- `config.test.json` - Template without secrets

## Stage Files

```bash
# Stage all files
git add .

# Or selectively stage
git add .gitignore .editorconfig .golangci.yml
git add LICENSE Makefile README.md CHANGELOG.md TESTING.md
git add go.mod go.sum
git add cmd/ config/ internal/ tests/
```

## Verify Staged Files

```bash
# Make sure no secrets are staged
git status

# Check for sensitive data
git diff --cached | grep -i "api.*key\|secret\|password\|token"
```

## Create Initial Commit

```bash
git commit -m "Initial commit: Metron MVP

Features:
- Core domain models with full validation
- SQLite storage layer with tests
- Session manager with multi-child support
- Generic scheduler with break rules
- Aqara Cloud driver implementation
- REST API with authentication
- Device driver registry
- Aqara test CLI tool

Test Coverage: 48/48 tests passing
Architecture: TDD-first, modular, extensible"
```

## Create Repository on GitHub/GitLab

```bash
# Add remote
git remote add origin https://github.com/your-username/metron.git

# Push to remote
git branch -M main
git push -u origin main
```

## Post-commit Steps

1. **Protect sensitive branches**
   - Enable branch protection for `main`
   - Require pull requests for changes

2. **Set up CI/CD** (optional)
   - GitHub Actions workflow
   - Run tests on every push
   - Check code coverage

3. **Add repository secrets** (if using CI/CD)
   - `AQARA_APP_ID`
   - `AQARA_APP_KEY`
   - `AQARA_KEY_ID`
   - (Store as encrypted secrets in GitHub/GitLab)

## Verify Commit

```bash
# Check commit history
git log --oneline

# Verify no secrets in history
git log --all --full-history -- config.json

# List tracked files
git ls-tree -r main --name-only
```

## Future Commits

Follow conventional commit format:

```bash
# Features
git commit -m "feat: add Telegram bot integration"

# Bug fixes
git commit -m "fix: handle scheduler race condition"

# Documentation
git commit -m "docs: update API documentation"

# Tests
git commit -m "test: add integration tests for sessions"

# Refactoring
git commit -m "refactor: simplify storage interface"
```

## Tags

Tag releases using semantic versioning:

```bash
# Create annotated tag
git tag -a v0.1.0 -m "MVP release: Core features complete"

# Push tags
git push origin --tags
```

## Ready to Commit!

Your project is clean, tested, and ready for version control.

Remember to:
1. Double-check no secrets in commits
2. Review .gitignore before first push
3. Keep sensitive config files local only
