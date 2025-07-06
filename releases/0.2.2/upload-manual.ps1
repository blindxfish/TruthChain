# TruthChain Manual Upload Helper for Windows
# This script helps you upload deployment files to your VPS

Write-Host "========================================" -ForegroundColor Cyan
Write-Host "TruthChain Manual Upload Helper" -ForegroundColor Cyan
Write-Host "========================================" -ForegroundColor Cyan
Write-Host ""
Write-Host "Target VPS: 168.231.108.135" -ForegroundColor Yellow
Write-Host ""

# Check if deployment files exist
$deployFiles = @(
    "mainnet-deploy.sh",
    "mainnet-setup.sh", 
    "ssl-setup.sh"
)

Write-Host "Checking deployment files..." -ForegroundColor Blue
$missingFiles = @()

foreach ($file in $deployFiles) {
    if (Test-Path $file) {
        Write-Host "✓ Found: $file" -ForegroundColor Green
    } else {
        Write-Host "✗ Missing: $file" -ForegroundColor Red
        $missingFiles += $file
    }
}

if ($missingFiles.Count -gt 0) {
    Write-Host ""
    Write-Host "ERROR: Some deployment files are missing!" -ForegroundColor Red
    Write-Host "Please ensure all files are in the deploy/ directory." -ForegroundColor Red
    Read-Host "Press Enter to exit"
    exit 1
}

Write-Host ""
Write-Host "========================================" -ForegroundColor Cyan
Write-Host "Manual Upload Instructions" -ForegroundColor Cyan
Write-Host "========================================" -ForegroundColor Cyan
Write-Host ""

Write-Host "Since scp is not available, here are your options:" -ForegroundColor Yellow
Write-Host ""

Write-Host "OPTION 1: Use a Web-Based File Manager" -ForegroundColor Green
Write-Host "1. Install a web-based file manager on your VPS:" -ForegroundColor White
Write-Host "   - FileZilla Server (FTP)"
Write-Host "   - Web-based file manager (like FileGator)"
Write-Host "   - Or use your VPS provider's web interface"
Write-Host ""

Write-Host "OPTION 2: Use WinSCP (Recommended)" -ForegroundColor Green
Write-Host "1. Download WinSCP from: https://winscp.net/" -ForegroundColor White
Write-Host "2. Install and run WinSCP"
Write-Host "3. Connect to your VPS:"
Write-Host "   - Host: 168.231.108.135"
Write-Host "   - Username: root"
Write-Host "   - Password: [your VPS password]"
Write-Host "4. Upload these files to /tmp/ on your VPS:"
foreach ($file in $deployFiles) {
    Write-Host "   - $file" -ForegroundColor Cyan
}
Write-Host ""

Write-Host "OPTION 3: Use PuTTY/PSFTP" -ForegroundColor Green
Write-Host "1. Download PuTTY from: https://www.putty.org/" -ForegroundColor White
Write-Host "2. Use PSFTP (included with PuTTY) to upload files"
Write-Host "3. Or use PuTTY's built-in file transfer"
Write-Host ""

Write-Host "OPTION 4: Copy-Paste Method" -ForegroundColor Green
Write-Host "1. SSH into your VPS: ssh root@168.231.108.135" -ForegroundColor White
Write-Host "2. Create files manually and copy-paste content"
Write-Host "3. Use nano or vim to create the files"
Write-Host ""

Write-Host "========================================" -ForegroundColor Cyan
Write-Host "File Contents for Copy-Paste" -ForegroundColor Cyan
Write-Host "========================================" -ForegroundColor Cyan
Write-Host ""

Write-Host "If you choose OPTION 4, here are the commands to run on your VPS:" -ForegroundColor Yellow
Write-Host ""

Write-Host "1. SSH into your VPS:" -ForegroundColor White
Write-Host "   ssh root@168.231.108.135"
Write-Host ""

Write-Host "2. Create the deployment script:" -ForegroundColor White
Write-Host "   nano /tmp/mainnet-deploy.sh"
Write-Host "   # Copy-paste the content from mainnet-deploy.sh"
Write-Host ""

Write-Host "3. Create the setup script:" -ForegroundColor White
Write-Host "   nano /tmp/mainnet-setup.sh"
Write-Host "   # Copy-paste the content from mainnet-setup.sh"
Write-Host ""

Write-Host "4. Make them executable:" -ForegroundColor White
Write-Host "   chmod +x /tmp/mainnet-deploy.sh"
Write-Host "   chmod +x /tmp/mainnet-setup.sh"
Write-Host ""

Write-Host "5. Run the deployment:" -ForegroundColor White
Write-Host "   /tmp/mainnet-deploy.sh"
Write-Host ""

Write-Host "========================================" -ForegroundColor Cyan
Write-Host "Recommended: WinSCP Method" -ForegroundColor Cyan
Write-Host "========================================" -ForegroundColor Cyan
Write-Host ""

Write-Host "For the easiest experience, I recommend using WinSCP:" -ForegroundColor Yellow
Write-Host "1. Download WinSCP (free)" -ForegroundColor White
Write-Host "2. Connect to 168.231.108.135 with root credentials"
Write-Host "3. Drag and drop the deployment files to /tmp/"
Write-Host "4. Then SSH in and run the deployment"
Write-Host ""

Write-Host "Would you like me to show you the content of any specific file?" -ForegroundColor Green
Write-Host "Type 'mainnet-deploy' or 'mainnet-setup' to see the content:" -ForegroundColor White

$choice = Read-Host "Enter file name (or press Enter to exit)"

if ($choice -eq "mainnet-deploy") {
    Write-Host ""
    Write-Host "=== mainnet-deploy.sh content ===" -ForegroundColor Cyan
    Get-Content "mainnet-deploy.sh" | ForEach-Object { Write-Host $_ }
} elseif ($choice -eq "mainnet-setup") {
    Write-Host ""
    Write-Host "=== mainnet-setup.sh content ===" -ForegroundColor Cyan
    Get-Content "mainnet-setup.sh" | ForEach-Object { Write-Host $_ }
}

Write-Host ""
Write-Host "Good luck with your deployment!" -ForegroundColor Green
Read-Host "Press Enter to exit" 