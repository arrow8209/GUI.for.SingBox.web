# 公网部署 checklist

## 必备前提

- 一个域名（已 DNS A 记录指向你的服务器）
- 一台 Linux 服务器（Ubuntu / Debian / CentOS 任意）
- HTTPS 证书（Let's Encrypt 免费）
- Nginx 或 Caddy 反向代理
- （可选但推荐）Cloudflare 前置 CDN/WAF

## 启动应用

推荐用 systemd 管理：

```bash
sudo nano /etc/systemd/system/gui-singbox.service
```

systemd unit 内容：

```ini
[Unit]
Description=GUI for sing-box web
After=network.target

[Service]
Type=simple
User=singbox
WorkingDirectory=/opt/gui-singbox
ExecStart=/opt/gui-singbox/gui-singbox
Environment="BIND=127.0.0.1:22345"
Environment="ALLOWED_ORIGINS=https://panel.example.com"
Environment="SECURE_COOKIE=true"
Environment="SESSION_TTL=24h"
Restart=on-failure
RestartSec=5

[Install]
WantedBy=multi-user.target
```

启用 + 查看初始密码：

```bash
sudo systemctl daemon-reload
sudo systemctl enable --now gui-singbox
sudo journalctl -u gui-singbox -f
```

记录 stderr 中显示的 `Initial admin password: <随机串>`，立即在浏览器登录后改密。

## Nginx 反代

`/etc/nginx/sites-available/panel.example.com`：

```nginx
server {
    listen 443 ssl http2;
    server_name panel.example.com;

    ssl_certificate /etc/letsencrypt/live/panel.example.com/fullchain.pem;
    ssl_certificate_key /etc/letsencrypt/live/panel.example.com/privkey.pem;
    ssl_protocols TLSv1.2 TLSv1.3;

    add_header Strict-Transport-Security "max-age=63072000" always;
    add_header X-Frame-Options DENY always;

    location / {
        proxy_pass http://127.0.0.1:22345;
        proxy_http_version 1.1;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;

        # WebSocket upgrade
        proxy_set_header Upgrade $http_upgrade;
        proxy_set_header Connection "upgrade";

        # WS 长连接
        proxy_read_timeout 300s;
    }
}

server {
    listen 80;
    server_name panel.example.com;
    return 301 https://$host$request_uri;
}
```

启用：

```bash
sudo ln -s /etc/nginx/sites-available/panel.example.com /etc/nginx/sites-enabled/
sudo nginx -t && sudo systemctl reload nginx
sudo certbot --nginx -d panel.example.com
```

## Caddy 反代（更简单，自动 HTTPS）

`/etc/caddy/Caddyfile`：

```caddy
panel.example.com {
    reverse_proxy 127.0.0.1:22345
    encode gzip
}
```

```bash
sudo systemctl reload caddy
```

Caddy 自动申请并续期 Let's Encrypt 证书。

## 防火墙

```bash
sudo ufw allow 22/tcp        # ssh
sudo ufw allow 80/tcp        # http (跳转到 https)
sudo ufw allow 443/tcp       # https
sudo ufw deny 22345/tcp      # 应用端口对外屏蔽
sudo ufw enable
```

## 上线前 checklist

- [ ] DNS 解析正常
- [ ] HTTPS 证书有效（`curl -I https://panel.example.com` 返回 200）
- [ ] 应用监听 127.0.0.1（`ss -ltnp | grep 22345` 显示 `127.0.0.1:22345` 而不是 `*:22345`）
- [ ] 22345 端口外部不可达（`curl http://公网IP:22345` 应连接超时）
- [ ] 已用 stderr 中的初始密码登录
- [ ] 已修改默认密码（强密码 ≥ 12 字符，含字母数字符号）
- [ ] `data/auth.yaml` 权限 600（`ls -la data/auth.yaml` → `-rw-------`）
- [ ] `data/.cache/initial-password.txt` 已被自动删除
- [ ] `ALLOWED_ORIGINS` 设为实际域名（不要留默认 loopback）
- [ ] systemd 服务自启、重启正常

## 进阶建议

- **Cloudflare 前置**：DNS 走 Cloudflare 橙云，可获 DDoS 防护与 WAF。
- **额外 IP 白名单**：Nginx 加 `allow X.X.X.X; deny all;` 限制访问 IP。
- **额外 Basic Auth**：在反代层加一层 HTTP Basic Auth 作为深度防御。
- **fail2ban**：监控应用日志中的 `429 too many attempts`，自动 ban IP。
- **日志归档**：`journalctl -u gui-singbox > /var/log/gui-singbox.log` 定期归档。
- **审计日志**（未来）：可加 `data/audit.log` 记录 exec/io/net 操作。

## 故障排查

| 现象 | 原因 | 处理 |
|------|------|------|
| 登录返回 401 | 密码错误 / 触发限速 | 等 5 分钟再试；查 `journalctl -u gui-singbox` |
| WebSocket 连不上 | Origin 不在白名单 | 检查 `ALLOWED_ORIGINS`，确认含访问域名 |
| 浏览器报 CSRF 403 | sessionStorage 丢失 csrf_token | 重新登录 |
| Core Proxy 400 "no active profile" | 前端未调 select-profile | 在 UI 切换 profile，前端会自动同步 |
| `data/auth.yaml` 损坏 | 手动编辑出错 | 删除文件，重启服务，新密码会显示在 stderr |
