package api

const indexHTML = `<!DOCTYPE html>
<html lang="zh-CN">
<head>
  <meta charset="UTF-8">
  <meta name="viewport" content="width=device-width, initial-scale=1.0">
  <title>EMOS Go Rewrite</title>
  <style>
    :root {
      color-scheme: dark;
      --bg: #0b1220;
      --panel: #111827;
      --panel-2: #0f172a;
      --line: #243041;
      --text: #e5e7eb;
      --muted: #94a3b8;
      --green: #10b981;
      --blue: #3b82f6;
      --cyan: #06b6d4;
    }
    * { box-sizing: border-box; }
    body {
      margin: 0;
      font-family: Arial, Helvetica, sans-serif;
      background: var(--bg);
      color: var(--text);
    }
    .container {
      max-width: 1200px;
      margin: 0 auto;
      padding: 24px;
    }
    .header {
      display: flex;
      justify-content: space-between;
      align-items: center;
      margin-bottom: 20px;
    }
    .title { font-size: 24px; font-weight: 700; }
    .sub { color: var(--muted); font-size: 13px; margin-top: 4px; }
    .grid {
      display: grid;
      grid-template-columns: 320px 1fr;
      gap: 20px;
    }
    .card {
      background: var(--panel);
      border: 1px solid var(--line);
      border-radius: 14px;
      padding: 18px;
    }
    .card h2 {
      margin: 0 0 14px;
      font-size: 15px;
    }
    label {
      display: block;
      font-size: 13px;
      color: var(--muted);
      margin-bottom: 8px;
    }
    input {
      width: 100%;
      border-radius: 10px;
      border: 1px solid var(--line);
      background: var(--panel-2);
      color: var(--text);
      padding: 10px 12px;
      margin-bottom: 12px;
    }
    button {
      width: 100%;
      border: 0;
      border-radius: 10px;
      padding: 11px 14px;
      cursor: pointer;
      background: var(--blue);
      color: white;
      font-weight: 700;
    }
    button:hover { opacity: 0.92; }
    .stats {
      display: grid;
      grid-template-columns: repeat(3, 1fr);
      gap: 12px;
      margin-bottom: 16px;
    }
    .stat {
      background: var(--panel-2);
      border: 1px solid var(--line);
      border-radius: 12px;
      padding: 14px;
    }
    .stat .label { color: var(--muted); font-size: 12px; }
    .stat .value { margin-top: 8px; font-size: 20px; font-weight: 700; }
    .current {
      background: var(--panel-2);
      border: 1px solid var(--line);
      border-radius: 12px;
      padding: 14px;
      margin-bottom: 16px;
    }
    .current .line { margin-bottom: 8px; }
    .current .key { color: var(--muted); font-size: 12px; }
    .current .val { margin-top: 4px; }
    .table {
      width: 100%;
      border-collapse: collapse;
      overflow: hidden;
      border-radius: 12px;
      border: 1px solid var(--line);
    }
    .table th, .table td {
      padding: 12px;
      border-bottom: 1px solid var(--line);
      text-align: left;
      font-size: 13px;
      vertical-align: top;
    }
    .table th {
      background: var(--panel-2);
      color: var(--muted);
    }
    .badge {
      display: inline-block;
      padding: 4px 8px;
      border-radius: 999px;
      font-size: 12px;
      font-weight: 700;
      background: #1f2937;
    }
    .pending { color: #fbbf24; }
    .running { color: #60a5fa; }
    .success { color: #34d399; }
    .footer-note {
      margin-top: 12px;
      color: var(--muted);
      font-size: 12px;
    }
    @media (max-width: 900px) {
      .grid { grid-template-columns: 1fr; }
      .stats { grid-template-columns: 1fr; }
    }
  </style>
</head>
<body>
  <div class="container">
    <div class="header">
      <div>
        <div class="title">EMOS Go Rewrite</div>
        <div class="sub">最小可运行任务面板，支持加任务、状态查询、SSE 实时刷新</div>
      </div>
    </div>

    <div class="grid">
      <div class="card">
        <h2>创建任务</h2>
        <label for="taskName">任务名称</label>
        <input id="taskName" placeholder="例如：测试视频-01.mkv">
        <button id="enqueueBtn">加入队列</button>
        <div class="footer-note" id="formMsg">等待操作</div>
      </div>

      <div>
        <div class="stats">
          <div class="stat">
            <div class="label">队列长度</div>
            <div class="value" id="queueSize">0</div>
          </div>
          <div class="stat">
            <div class="label">运行状态</div>
            <div class="value" id="workerRunning">idle</div>
          </div>
          <div class="stat">
            <div class="label">任务总数</div>
            <div class="value" id="taskCount">0</div>
          </div>
        </div>

        <div class="current">
          <div class="line">
            <div class="key">当前任务 ID</div>
            <div class="val" id="currentTaskId">-</div>
          </div>
          <div class="line">
            <div class="key">当前状态流</div>
            <div class="val" id="streamState">connecting...</div>
          </div>
        </div>

        <div class="card">
          <h2>任务列表</h2>
          <table class="table">
            <thead>
              <tr>
                <th>名称</th>
                <th>状态</th>
                <th>阶段</th>
                <th>状态文案</th>
                <th>错误</th>
              </tr>
            </thead>
            <tbody id="taskTableBody">
              <tr><td colspan="5">暂无任务</td></tr>
            </tbody>
          </table>
        </div>
      </div>
    </div>
  </div>

  <script>
    const taskName = document.getElementById('taskName');
    const enqueueBtn = document.getElementById('enqueueBtn');
    const formMsg = document.getElementById('formMsg');
    const queueSize = document.getElementById('queueSize');
    const workerRunning = document.getElementById('workerRunning');
    const taskCount = document.getElementById('taskCount');
    const currentTaskId = document.getElementById('currentTaskId');
    const streamState = document.getElementById('streamState');
    const taskTableBody = document.getElementById('taskTableBody');

    function badgeClass(status) {
      if (status === 'running') return 'badge running';
      if (status === 'success') return 'badge success';
      if (status === 'pending') return 'badge pending';
      return 'badge';
    }

    function render(snapshot) {
      queueSize.textContent = String(snapshot.queue_size || 0);
      workerRunning.textContent = snapshot.worker_running ? 'running' : 'idle';
      taskCount.textContent = String((snapshot.tasks || []).length);
      currentTaskId.textContent = snapshot.current_task_id || '-';

      const tasks = snapshot.tasks || [];
      if (!tasks.length) {
        taskTableBody.innerHTML = '<tr><td colspan="5">暂无任务</td></tr>';
        return;
      }

      taskTableBody.innerHTML = tasks.slice().reverse().map(task => `
        <tr>
          <td>${escapeHtml(task.name || '')}</td>
          <td><span class="${badgeClass(task.status)}">${escapeHtml(task.status || '-')}</span></td>
          <td>${escapeHtml(task.stage || '-')}</td>
          <td>${escapeHtml(task.status_text || '-')}</td>
          <td>${escapeHtml(task.last_error || '-')}</td>
        </tr>
      `).join('');
    }

    function escapeHtml(value) {
      return String(value)
        .replaceAll('&', '&')
        .replaceAll('<', '<')
        .replaceAll('>', '>')
        .replaceAll('"', '"')
        .replaceAll("'", ''');
    }

    async function loadSnapshot() {
      const res = await fetch('/api/tasks');
      const data = await res.json();
      render(data);
    }

    async function enqueue() {
      const name = taskName.value.trim();
      if (!name) {
        formMsg.textContent = '请输入任务名称';
        return;
      }
      enqueueBtn.disabled = true;
      formMsg.textContent = '提交中...';
      try {
        const res = await fetch('/api/queue/add', {
          method: 'POST',
          headers: { 'Content-Type': 'application/json' },
          body: JSON.stringify({ name })
        });
        const data = await res.json();
        if (!res.ok) {
          formMsg.textContent = data.error || '提交失败';
          return;
        }
        taskName.value = '';
        formMsg.textContent = '已加入队列：' + data.name;
        await loadSnapshot();
      } catch (err) {
        formMsg.textContent = '请求失败：' + err;
      } finally {
        enqueueBtn.disabled = false;
      }
    }

    function startEvents() {
      const es = new EventSource('/api/events');
      es.onopen = () => {
        streamState.textContent = 'connected';
      };
      es.onmessage = (evt) => {
        try {
          const payload = JSON.parse(evt.data);
          if (payload.snapshot) {
            render(payload.snapshot);
          }
        } catch (_) {}
      };
      es.onerror = () => {
        streamState.textContent = 'reconnecting...';
      };
    }

    enqueueBtn.addEventListener('click', enqueue);
    taskName.addEventListener('keydown', (e) => {
      if (e.key === 'Enter') enqueue();
    });

    loadSnapshot();
    startEvents();
  </script>
</body>
</html>
`
