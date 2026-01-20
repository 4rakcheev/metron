Metron Windows Agent
====================

The Metron Windows Agent enforces screen-time sessions on Windows PCs.
When no active session exists, the agent locks the workstation.

Installation
------------

1. Edit config.txt with your device settings (if not pre-filled):

   Required:
   - DEVICE_ID: Your device ID as registered in Metron
   - TOKEN: Agent authentication token from your Metron config
   - URL: Your Metron server URL (e.g., https://metron.example.com)

   Optional (uncomment in config.txt to change):
   - POLL_INTERVAL: How often to check server (default: 15 seconds)
   - GRACE_PERIOD: Wait time on network error before locking (default: 30 seconds)
   - LOG_LEVEL: debug, info, warn, error (default: info)
   - LOG_FORMAT: json or text (default: json)

2. Double-click install.bat
   - If prompted by UAC, click "Yes" to allow administrator access
   - The script will install and start the agent

3. The agent will start automatically at every user login.

File Locations
--------------

Binary:     C:\Program Files\Metron\metron-win-agent.exe
Log file:   C:\ProgramData\Metron\agent.log

Checking Status
---------------

Open PowerShell and run:

  Get-ScheduledTask -TaskName MetronAgent

View recent logs:

  Get-Content "C:\ProgramData\Metron\agent.log" -Tail 50

Updating
--------

To update the agent:
1. Download the new release package
2. Edit config.txt (or copy your existing config)
3. Run install.ps1 again - it will stop, update, and restart the agent

Uninstalling
------------

Open PowerShell as Administrator and run:

  # Stop and remove the scheduled task
  Unregister-ScheduledTask -TaskName "MetronAgent" -Confirm:$false

  # Remove installation files (optional)
  Remove-Item -Recurse "C:\Program Files\Metron"
  Remove-Item -Recurse "C:\ProgramData\Metron"

Troubleshooting
---------------

Agent not starting:
- Check logs at C:\ProgramData\Metron\agent.log
- Verify config.txt has correct URL and token
- Ensure the server is reachable: ping your-server.com

Agent locks immediately:
- Verify a session is active for this device in Metron
- Check that DEVICE_ID matches the device registered in Metron
- Parent can enable bypass mode via Telegram bot to temporarily disable enforcement

Network errors:
- Agent has a 30-second grace period for network issues
- After grace period expires, it fails closed (locks workstation)
- Check firewall rules allow outbound HTTPS to Metron server

Support
-------

For issues, check the Metron documentation or contact your administrator.
