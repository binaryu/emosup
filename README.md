
# EMOS PRO Panel

##  项目简介

EMOS PRO Panel 是一个专为 **EMOS** 设计的直观 **Web UI 面板**，旨在极大地简化和加速视频资源的批量上传过程。它通过无缝集成 **OpenList/Alist** 和 **Aria2**，为您提供从扫描、下载到自动匹配并上传视频的一站式解决方案，让您告别繁琐的手动操作，大幅提升内容管理效率。

## ✨ 核心功能

*   **直观的 Web UI 界面**：通过友好的网页界面轻松管理和批量上传视频。
*   **无缝集成 OpenList/Alist**：智能扫描您 OpenList/Alist 目录中的视频文件，无需手动收集。
*   **高速下载支持**：利用 Aria2 进行文件下载，确保视频资源快速准备就绪。
*   **智能文件名解析**：自动识别文件名中的季号和集号，简化剧集匹配过程。
*   **实时任务监控**：提供详细的任务状态监控和日志输出，随时掌握上传进度。
*   **自动化工作流**：将视频扫描、下载、匹配、上传等多个步骤整合为一套流畅的自动化流程。

## ⚙️ 核心依赖项

EMOS PRO Panel 依赖于以下外部服务才能正常工作。请确保您已预先安装并配置好它们：

1.  **OpenList / Alist**
    *   **功能**: 用于列出和管理您的文件，以便面板可以扫描到视频资源。
    *   **安装**: 请参考官方文档 [OpenList 安装教程](https://doc.oplist.org/guide/installation/script)。
    *   **注意**: 您的 OpenList/Alist 服务需要**可访问**，并且如果设置了认证，您需要提供相应的 Token。

2.  **Aria2**
    *   **功能**: 一个轻量级、多协议、多源的下载工具，面板将利用它来下载视频。
    *   **安装**:
        ```bash
        apt update && apt install -y wget curl ca-certificates 
        wget -N git.io/aria2.sh && chmod +x aria2.sh
        ./aria2.sh 
        ```
    *   **注意**: 确保您的 Aria2 RPC 服务已**启动并监听**，并且您知道其 RPC URL 和可能需要的 Secret。

## 快速开始 (推荐：一键部署脚本)

对于 Linux 用户，我们提供了一个便捷的一键部署脚本，可以快速安装并启动 EMOS PRO Panel。

**前置依赖**：您的系统需要安装 `curl` 和 `jq`。
```bash
sudo apt-get update && sudo apt-get install -y curl jq
```

**提示**:
*   此脚本会自动下载并安装本项目，并可能创建服务。
*   在运行之前，请务必阅读并理解[部署脚本的内容](https://github.com/binaryu/emosup/blob/main/deploy.sh)，以确保其符合您的期望。
*   请确保您已经安装并配置好上述**核心依赖项 (OpenList/Alist, Aria2)**。

**运行部署脚本**:
```bash
curl -sSL https://raw.githubusercontent.com/binaryu/emosup/main/deploy.sh | sudo bash
```
脚本运行完成后，EMOS PRO Panel 将默认在 `12345` 端口启动。您可以通过访问 `http://你的服务器IP:12345` 来访问面板。

## 手动安装 (开发者)

如果您希望从源代码运行或进行开发，请遵循以下步骤。

1.  **克隆项目仓库**:
    ```bash
    git clone https://github.com/binaryu/emosup.git
    cd emosup
    ```

2.  **安装 Python 依赖**:
    ```bash
    pip install -r requirements.txt
    ```

##  使用指南

1.  **启动 EMOS PRO Panel**:

    *   如果您使用了一键部署脚本，面板通常会作为服务自动启动。
    *   如果您是手动安装，可以通过以下命令启动主程序：
        ```bash
        # 默认在 12345 端口启动
        python main.py

        # 或者指定一个不同的端口，例如 8080
        python main.py --port 8080
        ```

2.  **访问 Web 面板**:

    打开您的浏览器，访问 `http://127.0.0.1:12345` (或您指定的端口，如果是远程服务器，请将 `127.0.0.1` 替换为服务器 IP 地址)。

3.  **配置参数**:

    在 Web 面板中，您需要填写以下核心参数。请确保这些信息准确无误，因为它们是面板正常工作的基础。

    | 参数                  | 说明                                         | 示例                            | 备注                                                      |
    | :-------------------- | :------------------------------------------- | :------------------------------ | :-------------------------------------------------------- |
    | **EMOS Token**        | 您的 EMOS API 访问密钥。                     | `1111_xxxxxxxx`           | **必需**                                                  |
    | **TMDB ID**           | 要上传的影视剧在 TMDB 上的唯一标识符 (ID)。  | `12345`                         | **必需**，用于获取剧集元数据进行匹配。                    |
    | **OpenList/Alist Base URL** | 您的 OpenList/Alist 服务的基础地址。         | `http://your_alist_ip:5244`     | **必需**，确保可以访问。                                  |
    | **OpenList/Alist Token**  | 您的 OpenList/Alist API 令牌。                 | `your_alist_api_token`          | 如果 OpenList/Alist 开启了认证，则**必需**。             |
    | **OPENLIST路径**      | 要扫描的视频文件在 OpenList/Alist 中的根路径。 | `/Movies/YourShowName`          | **必需**，面板会在此路径下递归查找视频文件。              |
    | **Aria2 RPC URL**     | 您的 Aria2 RPC 服务地址。                    | `http://127.0.0.1:6800/jsonrpc` | **必需**，确保 Aria2 RPC 服务已启动并监听。               |
    | **Aria2 RPC Secret**  | 您的 Aria2 RPC 密钥。                        | `your_aria2_secret`             | 如果 Aria2 RPC 开启了认证，则**必需**。                   |

4.  **操作流程**:

    *   填写完所有必要的配置参数后。
    *   点击“**扫描**”按钮，面板会自动从 OpenList/Alist 中查找并列出指定路径下的视频文件。
    *   勾选您需要上传的文件。
    *   点击“**开始上传**”按钮，面板将开始执行下载、匹配和上传任务。
    *   您可以在任务状态区域查看实时进度和日志输出。

## 项目结构

```
.
├── main.py             # 项目主入口，用于启动 FastAPI 应用
├── requirements.txt    # Python 依赖清单
├── deploy.sh           # 一键部署脚本 (Linux)
├── app/                # 应用核心目录
│   ├── __init__.py
│   ├── main.py         # FastAPI 应用定义、API 路由及网页服务
│   ├── tasks.py        # 核心任务调度和处理逻辑
│   ├── upload.py       # 文件上传到 EMOS 的具体逻辑
│   ├── openlist.py     # OpenList/Alist 客户端及其API交互逻辑
│   ├── aria2.py        # Aria2 RPC 客户端及下载管理逻辑
│   ├── clients.py      # EMOS API 客户端封装
│   ├── config.py       # 应用配置管理
│   ├── utils.py        # 常用工具函数集
│   └── templates/      # 前端页面模板目录
│       └── index.html  # Web UI 主页面
└── ...                 # 其他可能的文件或目录
```