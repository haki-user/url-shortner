param(
    [string]$ResourceGroup = "tinyurl-student-rg",
    [string]$VmName = "tinyurl-utility-vm"
)

$ErrorActionPreference = "Stop"

$script = @'
set -e
export DEBIAN_FRONTEND=noninteractive

apt-get update -y
apt-get install -y curl redis-server

cat >/etc/redis/redis.conf <<'REDISCONF'
bind 0.0.0.0
port 6379
protected-mode no
supervised systemd
daemonize no
save ""
appendonly no
maxmemory 128mb
maxmemory-policy allkeys-lru
tcp-keepalive 60
timeout 0
loglevel notice
REDISCONF

systemctl enable redis-server
systemctl restart redis-server

if ! command -v tailscale >/dev/null 2>&1; then
  curl -fsSL https://tailscale.com/install.sh | sh
fi

cat >/etc/sysctl.d/99-tinyurl-tailscale.conf <<'SYSCTL'
net.ipv4.ip_forward = 1
net.ipv6.conf.all.forwarding = 1
SYSCTL

sysctl --system >/dev/null
systemctl enable --now tailscaled

redis-cli -h 127.0.0.1 ping
redis-cli -h 127.0.0.1 config get maxmemory
redis-cli -h 127.0.0.1 config get maxmemory-policy
tailscale version
systemctl is-active tailscaled
'@

$tempScript = New-TemporaryFile
try {
    Set-Content -Path $tempScript -Value $script -NoNewline

    az vm run-command invoke `
        --resource-group $ResourceGroup `
        --name $VmName `
        --command-id RunShellScript `
        --scripts "@$tempScript" `
        --query "value[].message" `
        --output tsv
}
finally {
    Remove-Item $tempScript -Force
}
