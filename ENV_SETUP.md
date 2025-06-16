# 🌍 Environment Setup Guide

## Quick Start

### 1. Copy example config:
```bash
cp env.example .env
```

### 2. Set your environment:
- **Development**: `APP_ENV=development` (default)
- **Production**: `APP_ENV=production`
- **Local**: `APP_ENV=local`

### 3. Update .env with your tokens:
```bash
# Required
TELEGRAM_BOT_TOKEN=your_telegram_bot_token_here

# Optional (will use defaults)
SOLANA_RPC_URL=https://your-fast-rpc.com
JITO_UUID=your-jito-uuid
```

## Environment Features

| Environment | Debug | Public RPC | Optimized |
|-------------|-------|------------|-----------|
| `development` | ✅ ON | ✅ Yes | ❌ No |
| `production` | ❌ OFF | ⚠️ Private Recommended | ✅ Yes |
| `local` | ✅ ON | ✅ Yes | ❌ No |

## Quick Switch Commands

### Windows:
```cmd
env.bat dev    # Development
env.bat prod   # Production  
env.bat local  # Local
```

### Unix/Linux/Mac:
```bash
./env.sh dev    # Development
./env.sh prod   # Production
./env.sh local  # Local
```

## Configuration Overview

When you start the bot, you'll see:
```
🌍 Environment: development
```

This shows which environment is active and loads appropriate settings.

## Important Notes

- 🔧 **Development**: Full debug output, slower but detailed
- 🚀 **Production**: Optimized for speed, minimal logging
- 🏠 **Local**: Same as development, for local testing

Never share your `.env` file - it contains sensitive tokens! 