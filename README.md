# EMOS Upload Panel

一个用于 EMOS 的 Web UI 面板，方便通过网页批量上传视频资源。

## 主要功能

-   **Web UI 操作**: 提供简洁的网页界面，方便进行批量上传。
-   **文件源支持**: 支持从 OpenList/Alist 扫描视频文件作为上传源。
-   **高效传输**: 集成 Aria2 进行高速下载。
-   **智能匹配**: 自动识别文件名中的季号和集号。

## 前置依赖

**在使用此面板之前，您必须已经在服务器上安装并运行了以下两个服务：**

1.  **OpenList / Alist**: 用于提供文件列表。
2.  **Aria2**: 用于处理下载任务。

<details>
<summary>点击查看 OpenList 和 Aria2 的参考安装说明</summary>

-   **OpenList / Alist**
    -   **功能**: 用于列出和管理您的文件，以便面板可以扫描到视频资源。
    -   **安装**: 请参考官方文档 [OpenList 安装教程](https://doc.oplist.org/guide/installation/script)。
    -   **注意**: 您的 OpenList/Alist 服务需要**可访问**，并且如果设置了认证，您需要提供相应的 Token。

-   **Aria2**
    -   **功能**: 一个轻量级、多协议、多源的下载工具，面板将利用它来下载视频。
    -   **安装 (Debian/Ubuntu)**:
        ```bash
        sudo apt update && sudo apt install -y wget curl ca-certificates
        wget -N git.io/aria2.sh && chmod +x aria2.sh
        sudo ./aria2.sh # 以 root 权限运行以进行系统级安装
        ```
    -   **注意**: 确保您的 Aria2 RPC 服务已**启动并监听**，并且您知道其 RPC URL 和可能需要的 Secret。
</details>

## 快速开始 (推荐)

对于 Linux 用户，推荐使用一键部署脚本。

**前提**: 请确保您的服务器已安装 `curl` 和 `jq`。
(例如: `sudo apt-get update && sudo apt-get install curl jq`)

**运行脚本**:
```bash
curl -sSL https://raw.githubusercontent.com/binaryu/emosup/main/deploy.sh | sudo bash
```
脚本将会引导您完成安装，并将可执行文件安装到 `/usr/local/bin/emosup`。最后，它会自动生成 `systemd` 服务配置，实现开机自启。

## 下载

对于 Windows 和其他用户，或者希望手动安装的用户，可以从 [**GitHub Releases**](https://github.com/binaryu/emosup/releases) 页面下载最新的预编译版本。

## 进阶使用

<details>
<summary><strong>开发者</strong></summary>

1.  克隆本项目到本地：
    ```bash
    git clone https://github.com/binaryu/emosup.git
    cd emosup
    ```

2.  安装 Python 依赖：
    ```bash
    pip install -r requirements.txt
    ```

3.  运行主程序：
    ```bash
    # 默认在 12345 端口启动
    python main.py

    # 或者指定一个不同的端口
    python main.py --port 8080
    ```
</details>

<details>
<summary><strong>Linux：手动部署为 Systemd 服务</strong></summary>

1.  **下载可执行文件**:
    从 [GitHub Releases](https://github.com/binaryu/emosup/releases) 页面下载最新的 Linux 可执行文件 (例如 `emosup-Linux-amd64`)。
    将其放置在一个合适的目录，例如 `/opt/emosup/`，并重命名为 `emosup`。
    ```bash
    sudo mkdir -p /opt/emosup
    sudo mv ./emosup-Linux-amd64 /opt/emosup/emosup
    sudo chmod +x /opt/emosup/emosup
    ```

2.  **创建 service 文件**:
    创建一个新的 service 文件：
    ```bash
    sudo nano /etc/systemd/system/emosup.service
    ```
    将以下内容复制进去。**注意**：如果您的可执行文件路径或用户名不同，请修改 `ExecStart` 和 `User` 字段。

    ```ini
    [Unit]
    Description=EMOS Upload Panel Service
    After=network.target

    [Service]
    Type=simple
    User=your_user_name  # 替换为运行此服务的用户名
    WorkingDirectory=/opt/emosup
    ExecStart=/opt/emosup/emosup --port 12345
    Restart=on-failure
    RestartSec=5s

    [Install]
    WantedBy=multi-user.target
    ```

3.  **管理服务**:
    ```bash
    # 重新加载 systemd 配置
    sudo systemctl daemon-reload

    # 启动服务
    sudo systemctl start emosup

    # 设置开机自启
    sudo systemctl enable emosup

    # 查看服务状态
    sudo systemctl status emosup

    # 查看实时日志
    sudo journalctl -u emosup -f
    ```
</details>
