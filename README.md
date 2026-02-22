# EMOS PRO Panel

一个用于 EMOS 的 Web UI 面板，方便通过网页批量上传视频资源。

## 功能

-   通过 Web UI 批量上传视频。
-   支持从 OpenList 扫描视频文件。
-   使用 Aria2 进行高速下载。
-   自动匹配文件名中的季号和集号。
-   提供任务状态监控和日志输出。

## 安装

1.  克隆本项目到本地：
    ```bash
    git clone https://github.com/binaryu/emosup.git
    cd emosup
    ```

2.  安装 Python 依赖：
    ```bash
    pip install -r requirements.txt
    ```
3.  你需要手动安装openlist和aria2，以便使用该面板

    openlist安装:https://doc.oplist.org/guide/installation/script
    
    aria2安装：
    ```bash
    apt install wget curl ca-certificates
    wget -N git.io/aria2.sh && chmod +x aria2.sh
    ```
## 使用

1.  运行主程序：
    ```bash
    python main.py
    ```

2.  打开浏览器，访问 `http://127.0.0.1:12345` 即可看到 Web 面板。

3.  在面板中填写以下参数：
    -   **EMOS Token**: 你的 EMOS API 访问密钥。
    -   **TMDB ID**: 要上传的影视剧的 TMDB ID。
    -   **OpenList Base URL**: 你的 OpenList/Alist 服务地址。
    -   **OpenList Token**: 你的 OpenList/Alist API 令牌。
    -   **OPENLIST路径**: 要扫描的视频文件在 OpenList/Alist 中的根路径。
    -   **Aria2 RPC URL**: 你的 Aria2 RPC 服务地址。
    -   **Aria2 RPC Secret**: 你的 Aria2 RPC 密钥。

4.  点击“扫描”按钮，面板会自动从 OpenList/Alist 中查找视频文件。

5.  勾选需要上传的文件，然后点击“开始上传”按钮。

## 项目结构

```
.
├── main.py             # 项目主入口
├── requirements.txt    # Python 依赖
├── app/                # 应用主目录
│   ├── __init__.py
│   ├── main.py         # FastAPI 应用定义和 API 路由
│   ├── tasks.py        # 核心任务处理逻辑
│   ├── upload.py       # 上传相关逻辑
│   ├── openlist.py     # OpenList 客户端和相关逻辑
│   ├── aria2.py        # Aria2 RPC 客户端
│   ├── clients.py      # EMOS API 客户端
│   ├── config.py       # 应用配置
│   ├── utils.py        # 工具函数
│   └── templates/
│       └── index.html  # 前端页面模板
└── ...