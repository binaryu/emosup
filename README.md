# EMOS Upload Panel

一个用于 EMOS 的 Web UI 面板，方便通过网页批量上传视频资源。

## 主要功能

-   **Web UI 操作**: 提供简洁的网页界面，方便进行批量上传。
-   **文件源支持**: 支持从 OpenList/Alist 扫描视频文件作为上传源。
-   **高效传输**: 集成 Aria2 进行高速下载。
-   **智能匹配**: 自动识别文件名中的季号和集号。

## 快速开始 (推荐)

对于 Linux 用户，推荐使用一键部署脚本。

**前提**: 请确保您的服务器已安装 `curl` 和 `jq`。
( `sudo apt-get update && sudo apt-get install curl jq`)

**运行脚本**:
```bash
curl -sSL https://raw.githubusercontent.com/binaryu/emosup/main/deploy.sh | sudo bash
```
脚本将会引导您完成安装，并将可执行文件安装到 `/usr/local/bin/emosup`。最后，它会自动生成 `systemd` 服务配置，您只需复制粘贴即可完成部署，实现开机自启。

## 下载

对于 Windows 和其他用户，或者希望手动安装的用户，可以从 [**GitHub Releases**](https://github.com/binaryu/emosup/releases) 页面下载最新的预编译版本。

## 进阶使用

<details>
<summary><strong>开发者：从源码运行</strong></summary>

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