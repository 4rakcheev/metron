# Testing Aqara Integration

This guide helps you test the Aqara Cloud API integration before running the full Metron application.

## Prerequisites

1. **Aqara Developer Account**: Sign up at [https://developer.aqara.com](https://developer.aqara.com)
2. **Create an App**: Get your `app_id`, `app_key`, and `key_id`
3. **Create Scenes**: Set up three scenes in the Aqara app:
   - PIN Entry scene (to allow TV access)
   - Warning scene (to show warning before timeout)
   - Power Off scene (to turn off TV)
4. **Scene IDs**: Note down the scene IDs from the Aqara Cloud platform

## Configuration

### Option 1: Using config file

Copy `config.test.json` to `config.json` and fill in your credentials:

```json
{
  "aqara": {
    "app_id": "your-actual-app-id",
    "app_key": "your-actual-app-key",
    "key_id": "your-actual-key-id",
    "base_url": "https://open-cn.aqara.com",
    "scenes": {
      "tv_pin_entry": "your-actual-pin-scene-id",
      "tv_warning": "your-actual-warning-scene-id",
      "tv_power_off": "your-actual-poweroff-scene-id"
    }
  }
}
```

### Option 2: Using environment variables

Set these environment variables:

```bash
export METRON_AQARA_APP_ID="your-app-id"
export METRON_AQARA_APP_KEY="your-app-key"
export METRON_AQARA_KEY_ID="your-key-id"
export METRON_AQARA_BASE_URL="https://open-cn.aqara.com"
export METRON_AQARA_TV_PIN_SCENE="your-pin-scene-id"
export METRON_AQARA_TV_WARNING_SCENE="your-warning-scene-id"
export METRON_AQARA_TV_POWEROFF_SCENE="your-poweroff-scene-id"

# These are required but not used for Aqara testing
export METRON_API_KEY="test-key"
export METRON_TELEGRAM_BOT_TOKEN="dummy-token"
```

## Build and Run

### Build the test tool:

```bash
cd /Users/tema/Pr/go/src/metron
go build -o bin/aqara-test ./cmd/aqara-test
```

### Test PIN entry scene:

```bash
./bin/aqara-test -action pin
```

### Test warning scene:

```bash
./bin/aqara-test -action warn
```

### Test power-off scene:

```bash
./bin/aqara-test -action off
```

### Use a specific config file:

```bash
./bin/aqara-test -config /path/to/config.json -action pin
```

## Expected Output

On success, you should see:

```
Testing Aqara Cloud API...
Action: pin
Base URL: https://open-cn.aqara.com

Triggering PIN entry scene: your-pin-scene-id

✅ Success! Scene your-pin-scene-id triggered successfully.
```

## Troubleshooting

### Error: "Invalid scene ID"
- Verify your scene IDs are correct in the Aqara Cloud console
- Scene IDs should match exactly

### Error: "Authentication failed" or "Invalid signature"
- Check that `app_id`, `app_key`, and `key_id` are correct
- Ensure no extra spaces in your credentials

### Error: "Network timeout"
- Check your internet connection
- Try a different `base_url` if you're in a different region
- China region: `https://open-cn.aqara.com`
- US region: `https://open-usa.aqara.com`
- Europe region: `https://open-ger.aqara.com`

### Error: "Config file not found"
- Make sure you've created `config.json` from `config.test.json`
- Or use environment variables as shown above

## What Gets Triggered?

When you run these tests:

- **PIN Entry**: Triggers your configured "allow access" scene
- **Warning**: Triggers your configured "time running out" notification scene
- **Power Off**: Triggers your configured "turn off device" scene

The actual behavior depends on what devices/actions you've configured in each scene in the Aqara app.

## Next Steps

Once Aqara integration works:

1. Set up child profiles in the database
2. Configure Telegram bot
3. Run the full Metron service
4. Test complete workflows (start session → warning → auto-stop)
