# EMOS Upload Panel
### 纯AI开发，目前处于测试阶段
一个用于 EMOS 的 Web UI 面板，方便通过网页批量上传视频资源。

## 主要功能

- **Web UI 操作**：提供简洁的网页界面，方便进行批量上传。
- **文件源支持**：支持从 OpenList/本地 扫描视频文件作为上传源。
- **高效传输**：集成 Aria2 进行高速下载。
- **智能匹配**：自动识别文件名中的季号和集号。

## 前置依赖

**在使用此面板之前，您必须已经在服务器上安装并运行了以下两个服务：**

1. **OpenList / Alist**：用于提供文件列表。
2. **Aria2**：用于处理下载任务。

<details>
<summary>点击查看 OpenList 和 Aria2 的参考安装说明</summary>

- **OpenList / Alist**
  - **功能**：用于列出和管理您的文件，以便面板可以扫描到视频资源。
  - **安装**：请参考官方文档 [OpenList 安装教程](https://doc.oplist.org/guide/installation/script)。
  - **注意**：您的 OpenList/Alist 服务需要**可访问**，并且如果设置了认证，您需要提供相应的 Token。

- **Aria2**
  - **功能**：一个轻量级、多协议、多源的下载工具，面板将利用它来下载视频。
  - **安装（Debian/Ubuntu）**：
    ```bash
    sudo apt update && sudo apt install -y wget curl ca-certificates
    wget -N git.io/aria2.sh && chmod +x aria2.sh
    sudo ./aria2.sh
    ```
  - **注意**：确保您的 Aria2 RPC 服务已**启动并监听**，并且您知道其 RPC URL 和可能需要的 Secret。
</details>

## 服务运行

下载已构建版本，直接把可执行文件托管成服务即可

### systemd

创建 `/etc/systemd/system/emosup.service`：

```ini
[Unit]
Description=EMOS Upload Panel
After=network.target

[Service]
Type=simple
User=运行用户
WorkingDirectory=/opt/emosup
ExecStart=/opt/emosup/emosup
Restart=always
RestartSec=5

[Install]
WantedBy=multi-user.target
```

启用：

```bash
sudo systemctl daemon-reload
sudo systemctl enable --now emosup
```

查看日志：

```bash
sudo journalctl -u emosup -f
```

## Docker 部署

1. 克隆项目：

   ```bash
   git clone https://github.com/binaryu/emosup.git
   cd emosup
   ```

2. 启动服务（自动构建镜像）：

   ```bash
   docker compose up -d --build
   ```

3. 访问面板：

   ```text
   http://服务器IP:12345
   ```

