# install.ps1 - Metron Windows Agent Installer
# Run via install.bat (double-click install.bat)

# Don't stop on errors - we want to see them and handle them
$ErrorActionPreference = "Continue"

function Write-Step {
    param([string]$Message)
    Write-Host "`n>>> $Message" -ForegroundColor Cyan
}

function Write-OK {
    param([string]$Message)
    Write-Host "    OK: $Message" -ForegroundColor Green
}

function Write-Err {
    param([string]$Message)
    Write-Host "    ERROR: $Message" -ForegroundColor Red
}

function Write-Info {
    param([string]$Message)
    Write-Host "    $Message" -ForegroundColor Gray
}

Write-Host "=== Metron Windows Agent Installer ===" -ForegroundColor Cyan
Write-Host "Script location: $PSScriptRoot"
Write-Host "PowerShell version: $($PSVersionTable.PSVersion)"
Write-Host ""

# ============================================
# Step 1: Read and validate config
# ============================================
Write-Step "Reading configuration..."

$ConfigFile = Join-Path $PSScriptRoot "config.txt"
Write-Info "Config file: $ConfigFile"

if (-not (Test-Path $ConfigFile)) {
    Write-Err "config.txt not found!"
    Write-Host "`nPlease create config.txt with your device settings."
    Write-Host "`nPress any key to exit..."
    $null = $Host.UI.RawUI.ReadKey("NoEcho,IncludeKeyDown")
    exit 1
}
Write-OK "Config file found"

# Parse config (simple KEY=VALUE format, ignoring comments and empty lines)
$Config = @{}
Get-Content $ConfigFile | ForEach-Object {
    $line = $_.Trim()
    if ($line -and -not $line.StartsWith("#") -and $line -match '^([^=]+)=(.*)$') {
        $key = $Matches[1].Trim()
        $value = $Matches[2].Trim()
        $Config[$key] = $value
        Write-Info "  $key = $value"
    }
}

# Validate required fields
$Required = @("DEVICE_ID", "TOKEN", "URL")
$Missing = @()
foreach ($Key in $Required) {
    if (-not $Config[$Key] -or $Config[$Key] -eq "your-agent-token-here" -or $Config[$Key] -eq "") {
        $Missing += $Key
    }
}

if ($Missing.Count -gt 0) {
    Write-Err "Missing or unconfigured values:"
    foreach ($Key in $Missing) {
        Write-Host "    - $Key" -ForegroundColor Red
    }
    Write-Host "`nPlease edit config.txt before running install."
    Write-Host "`nPress any key to exit..."
    $null = $Host.UI.RawUI.ReadKey("NoEcho,IncludeKeyDown")
    exit 1
}
Write-OK "Configuration validated"

$DeviceID = $Config["DEVICE_ID"]
$Token = $Config["TOKEN"]
$URL = $Config["URL"]

# ============================================
# Step 2: Define paths
# ============================================
Write-Step "Setting up paths..."

$InstallDir = "C:\Program Files\Metron"
$DataDir = "C:\ProgramData\Metron"
$Binary = "metron-win-agent.exe"
$TaskName = "MetronAgent"
$SourceBinary = Join-Path $PSScriptRoot $Binary

Write-Info "Source binary: $SourceBinary"
Write-Info "Install directory: $InstallDir"
Write-Info "Data directory: $DataDir"

# Verify binary exists
if (-not (Test-Path $SourceBinary)) {
    Write-Err "$Binary not found in $PSScriptRoot"
    Write-Host "`nMake sure metron-win-agent.exe is in the same folder as this script."
    Write-Host "`nPress any key to exit..."
    $null = $Host.UI.RawUI.ReadKey("NoEcho,IncludeKeyDown")
    exit 1
}
$BinarySize = (Get-Item $SourceBinary).Length / 1MB
Write-OK "Binary found ($([math]::Round($BinarySize, 2)) MB)"

# ============================================
# Step 3: Stop existing agent
# ============================================
Write-Step "Checking for existing agent..."

$ExistingTask = Get-ScheduledTask -TaskName $TaskName -ErrorAction SilentlyContinue
if ($ExistingTask) {
    Write-Info "Found existing scheduled task (State: $($ExistingTask.State))"
    if ($ExistingTask.State -eq "Running") {
        Write-Info "Stopping running agent..."
        Stop-ScheduledTask -TaskName $TaskName -ErrorAction SilentlyContinue
        Start-Sleep -Seconds 2
        Write-OK "Agent stopped"
    }
} else {
    Write-Info "No existing agent found (fresh install)"
}

# ============================================
# Step 4: Create directories
# ============================================
Write-Step "Creating directories..."

try {
    if (-not (Test-Path $InstallDir)) {
        New-Item -ItemType Directory -Path $InstallDir -Force | Out-Null
        Write-OK "Created $InstallDir"
    } else {
        Write-Info "$InstallDir already exists"
    }
} catch {
    Write-Err "Failed to create $InstallDir : $_"
}

try {
    if (-not (Test-Path $DataDir)) {
        New-Item -ItemType Directory -Path $DataDir -Force | Out-Null
        Write-OK "Created $DataDir"
    } else {
        Write-Info "$DataDir already exists"
    }
} catch {
    Write-Err "Failed to create $DataDir : $_"
}

# Set permissions on data directory (allow Users to write logs)
Write-Info "Setting permissions on data directory..."
try {
    $Acl = Get-Acl $DataDir
    # Use SID S-1-5-32-545 for BUILTIN\Users (works on all Windows languages)
    $UsersSid = New-Object System.Security.Principal.SecurityIdentifier("S-1-5-32-545")
    $Rule = New-Object System.Security.AccessControl.FileSystemAccessRule(
        $UsersSid,
        "Modify",
        "ContainerInherit,ObjectInherit",
        "None",
        "Allow"
    )
    $Acl.SetAccessRule($Rule)
    Set-Acl $DataDir $Acl
    Write-OK "Permissions set"
} catch {
    Write-Err "Failed to set permissions: $_"
}

# ============================================
# Step 5: Copy binary
# ============================================
Write-Step "Installing binary..."

$DestBinary = Join-Path $InstallDir $Binary
try {
    Copy-Item $SourceBinary $DestBinary -Force
    if (Test-Path $DestBinary) {
        Write-OK "Binary copied to $DestBinary"
    } else {
        Write-Err "Binary was not copied!"
    }
} catch {
    Write-Err "Failed to copy binary: $_"
}

# ============================================
# Step 6: Create scheduled task
# ============================================
Write-Step "Configuring scheduled task..."

$LogPath = Join-Path $DataDir "agent.log"
$Arguments = "-device-id `"$DeviceID`" -token `"$Token`" -url `"$URL`" -log-path `"$LogPath`""

# Add optional parameters if specified
if ($Config["POLL_INTERVAL"]) {
    $Arguments += " -poll-interval $($Config["POLL_INTERVAL"])"
}
if ($Config["GRACE_PERIOD"]) {
    $Arguments += " -grace-period $($Config["GRACE_PERIOD"])"
}
if ($Config["LOG_LEVEL"]) {
    $Arguments += " -log-level $($Config["LOG_LEVEL"])"
}
if ($Config["LOG_FORMAT"]) {
    $Arguments += " -log-format $($Config["LOG_FORMAT"])"
}

Write-Info "Executable: $DestBinary"
Write-Info "Arguments: $Arguments"

try {
    $Action = New-ScheduledTaskAction -Execute $DestBinary -Argument $Arguments
    Write-OK "Task action created"
} catch {
    Write-Err "Failed to create task action: $_"
}

try {
    # Trigger at logon for any user
    $Trigger = New-ScheduledTaskTrigger -AtLogOn
    Write-OK "Task trigger created (AtLogOn)"
} catch {
    Write-Err "Failed to create task trigger: $_"
}

try {
    # Run as the logged-on user with limited privileges
    # Use SID S-1-5-32-545 for BUILTIN\Users (works on all Windows languages)
    $UsersSid = New-Object System.Security.Principal.SecurityIdentifier("S-1-5-32-545")
    $UsersAccount = $UsersSid.Translate([System.Security.Principal.NTAccount]).Value
    Write-Info "Users group resolved to: $UsersAccount"
    $Principal = New-ScheduledTaskPrincipal -GroupId $UsersAccount -RunLevel Limited
    Write-OK "Task principal created"
} catch {
    Write-Err "Failed to create task principal: $_"
    Write-Info "Falling back to current user principal..."
    try {
        $Principal = New-ScheduledTaskPrincipal -UserId $env:USERNAME -LogonType Interactive -RunLevel Limited
        Write-OK "Task principal created (current user fallback)"
    } catch {
        Write-Err "Fallback also failed: $_"
    }
}

try {
    # Settings: restart on failure, run on battery, hidden
    $Settings = New-ScheduledTaskSettingsSet `
        -AllowStartIfOnBatteries `
        -DontStopIfGoingOnBatteries `
        -StartWhenAvailable `
        -RestartCount 3 `
        -RestartInterval (New-TimeSpan -Minutes 1) `
        -ExecutionTimeLimit (New-TimeSpan -Days 365) `
        -Hidden
    Write-OK "Task settings created (hidden)"
} catch {
    Write-Err "Failed to create task settings: $_"
}

# Remove existing task and recreate
Write-Info "Registering scheduled task..."
try {
    Unregister-ScheduledTask -TaskName $TaskName -Confirm:$false -ErrorAction SilentlyContinue

    Register-ScheduledTask `
        -TaskName $TaskName `
        -Action $Action `
        -Trigger $Trigger `
        -Principal $Principal `
        -Settings $Settings `
        -Description "Metron screen-time enforcement agent. Locks workstation when no active session." | Out-Null

    Write-OK "Scheduled task registered"
} catch {
    Write-Err "Failed to register scheduled task: $_"
}

# ============================================
# Step 7: Start the agent
# ============================================
Write-Step "Starting agent..."

try {
    Start-ScheduledTask -TaskName $TaskName
    Write-OK "Start command sent"
} catch {
    Write-Err "Failed to start task: $_"
}

# Wait and verify
Start-Sleep -Seconds 3

# ============================================
# Step 8: Verify installation
# ============================================
Write-Step "Verifying installation..."

# Check binary
if (Test-Path $DestBinary) {
    Write-OK "Binary installed at $DestBinary"
} else {
    Write-Err "Binary NOT found at $DestBinary"
}

# Check task
$Task = Get-ScheduledTask -TaskName $TaskName -ErrorAction SilentlyContinue
if ($Task) {
    Write-OK "Scheduled task exists (State: $($Task.State))"
    $Info = Get-ScheduledTaskInfo -TaskName $TaskName -ErrorAction SilentlyContinue
    if ($Info.LastRunTime) {
        Write-Info "Last run: $($Info.LastRunTime)"
    }
    if ($Info.LastTaskResult -ne $null) {
        Write-Info "Last result: $($Info.LastTaskResult)"
    }
} else {
    Write-Err "Scheduled task NOT found"
}

# Check if log file exists (means agent started and wrote something)
if (Test-Path $LogPath) {
    Write-OK "Log file created at $LogPath"
} else {
    Write-Info "Log file not yet created (agent may still be starting)"
}

# ============================================
# Summary
# ============================================
Write-Host "`n========================================" -ForegroundColor Green
Write-Host "Installation Complete" -ForegroundColor Green
Write-Host "========================================" -ForegroundColor Green
Write-Host ""
Write-Host "Installation directory: $InstallDir"
Write-Host "Log file: $LogPath"
Write-Host ""
Write-Host "The agent will start automatically at user login."
Write-Host ""
Write-Host "To check agent status:"
Write-Host "  Get-ScheduledTask -TaskName MetronAgent"
Write-Host ""
Write-Host "To view logs:"
Write-Host "  Get-Content `"$LogPath`" -Tail 50"
Write-Host ""
Write-Host "Press any key to exit..."
$null = $Host.UI.RawUI.ReadKey("NoEcho,IncludeKeyDown")
