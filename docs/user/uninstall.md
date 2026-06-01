# 卸载

按你部署/安装的方式选对应一节。**数据目录含 SQLite 数据库,删了不可恢复**——删之前想清楚是否要备份。

---

## 一、探针（agent，被监控的机器）

### 方式 A：面板远程卸载（推荐）

在网页/App 的探针列表里删除该探针时勾选"卸载"（API 为 `DELETE /api/agents/:id?uninstall=true`）。若探针在线，hub 会下发自卸载命令，探针停服务、删 systemd 单元、删自身二进制（best-effort）。

> 探针离线时无法远程自卸载——删除只会移除面板记录和 token，机器上的探针仍在跑，需用方式 B 手动清。

### 方式 B：在被监控机器上手动卸载

```bash
curl -fsSL "<hub>/install-agent.sh?token=<TOKEN>&agent_id=<AGENT_ID>" | bash -s -- --uninstall
```
`<hub>` 换成你的面板地址，`<TOKEN>`/`<AGENT_ID>` 用安装时那组。脚本会停掉并删除 `shiguang-agent` 服务与二进制。完成后记得**也在面板里删掉这条探针**。

---

## 二、服务端（hub）

### 方式 1：Docker

```bash
# docker run 部署
docker rm -f shiguang-vps                       # 停止并删除容器
docker rmi ghcr.io/carppond/fcvps-ui:latest      # （可选）删镜像
rm -rf ./data                                    # （可选，含数据库，不可恢复）

# docker compose 部署
docker compose down                              # 停止并删除容器
docker compose down -v                           # 连同卷一起删
rm -rf ./data                                    # 若用的是 ./data 绑定挂载
```

### 方式 2：一键脚本 `deploy.sh`

```bash
./scripts/deploy.sh --uninstall
```
交互式输入 VPS 连接信息后，脚本会远程：停止并禁用 `shiguang-vps` 服务、删除其 systemd 单元、删除 nginx 站点配置并 reload、删除二进制与前端文件。随后**逐项单独确认**（默认都不删，避免误伤）：

- **数据目录** `${REMOTE_DIR}/data`（含 SQLite 数据库）——默认保留。
- **SSL 证书**——脚本从 nginx 配置**自动探测域名**，问你是否 `certbot delete`（删后再签有 Let's Encrypt 频率限制，故默认不删）。
- **ufw 放行规则**——自动探测面板访问端口并询问删除，**始终排除 SSH 端口**，绝不会把你锁在门外。

唯一无法自动处理的是**被监控机器上的探针**：它们在别的机器上，hub 在自我拆除时已无法远程下发卸载命令——需按本文第一节「探针」在各机器手动卸载。

### 方式 3：手动二进制

```bash
# 若注册了 systemd 服务
sudo systemctl stop shiguang-vps
sudo systemctl disable shiguang-vps
sudo rm -f /etc/systemd/system/shiguang-vps.service
sudo systemctl daemon-reload

# 删除程序文件（数据目录按需保留/删除）
rm -f /opt/shiguang-vps/hub
rm -rf /opt/shiguang-vps/data          # 含数据库，不可恢复

# 若配了 nginx 反代
sudo rm -f /etc/nginx/sites-{available,enabled}/shiguang-vps
sudo nginx -t && sudo systemctl reload nginx
```

---

## 三、移动端 App

- **Android**：长按图标 → 卸载（或设置 → 应用 → 拾光VPS → 卸载）。
- **iOS**：长按图标 → 移除 App。用 Sideloadly/AltStore 自签的也一样直接删。

---

> 重新安装见 [快速开始](./quickstart.md) 与 [README](https://github.com/carppond/FCVPS-UI#readme) 的部署章节。
