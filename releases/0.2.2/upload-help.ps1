Write-Host "========================================" -ForegroundColor Cyan
Write-Host "TruthChain VPS Upload Help" -ForegroundColor Cyan
Write-Host "========================================" -ForegroundColor Cyan
Write-Host ""
Write-Host "Target VPS: 168.231.108.135" -ForegroundColor Yellow
Write-Host ""

Write-Host "Since scp is not available, here are your upload options:" -ForegroundColor Yellow
Write-Host ""

Write-Host "OPTION 1: WinSCP (Easiest)" -ForegroundColor Green
Write-Host "1. Download WinSCP from: https://winscp.net/" -ForegroundColor White
Write-Host "2. Install and run WinSCP"
Write-Host "3. Connect to: 168.231.108.135 (root user)"
Write-Host "4. Upload these files to /tmp/:"
Write-Host "   - mainnet-deploy.sh" -ForegroundColor Cyan
Write-Host "   - mainnet-setup.sh" -ForegroundColor Cyan
Write-Host "   - ssl-setup.sh" -ForegroundColor Cyan
Write-Host ""

Write-Host "OPTION 2: Copy-Paste Method" -ForegroundColor Green
Write-Host "1. SSH into your VPS: ssh root@168.231.108.135" -ForegroundColor White
Write-Host "2. Create files manually:"
Write-Host "   nano /tmp/mainnet-deploy.sh"
Write-Host "   nano /tmp/mainnet-setup.sh"
Write-Host "3. Copy-paste the content from the files"
Write-Host "4. Make executable: chmod +x /tmp/*.sh"
Write-Host ""

Write-Host "OPTION 3: Use your VPS provider's web interface" -ForegroundColor Green
Write-Host "Many VPS providers have web-based file managers" -ForegroundColor White
Write-Host "Check your VPS control panel for file upload options"
Write-Host ""

Write-Host "After uploading, run these commands on your VPS:" -ForegroundColor Yellow
Write-Host "1. chmod +x /tmp/mainnet-deploy.sh" -ForegroundColor White
Write-Host "2. chmod +x /tmp/mainnet-setup.sh"
Write-Host "3. /tmp/mainnet-deploy.sh"
Write-Host "4. sudo -u truthchain /tmp/mainnet-setup.sh"
Write-Host "5. sudo systemctl start truthchain"
Write-Host ""

Write-Host "Need the file contents for copy-paste?" -ForegroundColor Green
Write-Host "Type 'deploy' or 'setup' to see the content:" -ForegroundColor White

$choice = Read-Host "Enter choice"

if ($choice -eq "deploy") {
    Write-Host ""
    Write-Host "=== mainnet-deploy.sh content ===" -ForegroundColor Cyan
    if (Test-Path "mainnet-deploy.sh") {
        Get-Content "mainnet-deploy.sh"
    } else {
        Write-Host "File not found!" -ForegroundColor Red
    }
} elseif ($choice -eq "setup") {
    Write-Host ""
    Write-Host "=== mainnet-setup.sh content ===" -ForegroundColor Cyan
    if (Test-Path "mainnet-setup.sh") {
        Get-Content "mainnet-setup.sh"
    } else {
        Write-Host "File not found!" -ForegroundColor Red
    }
}

Write-Host ""
Write-Host "Good luck with your deployment!" -ForegroundColor Green
Read-Host "Press Enter to exit" 