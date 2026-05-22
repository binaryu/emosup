<template>
  <div class="page-shell">
    <aside class="sidebar" :class="{ 'sidebar-open': sidebarOpen }">
      <div class="brand">
        <span class="brand-badge">EM</span>
        <div class="brand-text">
          <strong>Emos Upload Panel</strong>
          <p>Queue-first MVP</p>
        </div>
      </div>

      <nav class="nav-list">
        <RouterLink v-for="item in navItems" :key="item.to" :to="item.to" class="nav-link" @click="closeSidebar">
          {{ item.label }}
        </RouterLink>
      </nav>
    </aside>

    <main class="page-main">
      <div v-if="isMobile" class="mobile-bar">
        <button class="menu-toggle" @click="sidebarOpen = !sidebarOpen">
          ☰
        </button>
        <span class="mobile-title">Emosup</span>
      </div>
      <RouterView />
    </main>
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted, onUnmounted } from 'vue'

const navItems = [
  { label: '配置页', to: '/config' },
  { label: 'OpenList 浏览', to: '/openlist' },
  { label: '本地浏览', to: '/local' },
  { label: '扫描结果', to: '/scans' },
  { label: '任务队列', to: '/tasks' },
]

const sidebarOpen = ref(false)
const isMobile = ref(false)

function checkMobile() {
  isMobile.value = window.innerWidth <= 960
  if (!isMobile.value) {
    sidebarOpen.value = false
  }
}

function closeSidebar() {
  sidebarOpen.value = false
}

onMounted(() => {
  checkMobile()
  window.addEventListener('resize', checkMobile)
})

onUnmounted(() => {
  window.removeEventListener('resize', checkMobile)
})
</script>

<style scoped>
.sidebar {
  width: 280px;
  min-width: 280px;
  padding: 24px;
  background: linear-gradient(180deg, #173c32 0%, #102b24 100%);
  color: #f6f2e9;
  transition: transform 0.25s ease, opacity 0.25s ease;
  z-index: 10;
}

.brand {
  display: flex;
  align-items: center;
  gap: 14px;
  margin-bottom: 28px;
}

.brand strong {
  display: block;
  font-size: 18px;
}

.brand p {
  margin: 6px 0 0;
  color: rgba(246, 242, 233, 0.72);
  font-size: 13px;
}

.brand-badge {
  display: grid;
  place-items: center;
  width: 44px;
  height: 44px;
  border-radius: 14px;
  background: rgba(255, 255, 255, 0.1);
  font-weight: 700;
  flex-shrink: 0;
}

.nav-list {
  display: flex;
  flex-direction: column;
  gap: 10px;
}

.nav-link {
  padding: 12px 14px;
  border-radius: 14px;
  color: rgba(246, 242, 233, 0.88);
  transition: background-color 0.2s ease;
  white-space: nowrap;
}

.nav-link.router-link-active {
  background: rgba(255, 255, 255, 0.12);
  color: #fff;
}

.nav-link:hover {
  background: rgba(255, 255, 255, 0.08);
}

.mobile-bar {
  display: none;
  align-items: center;
  gap: 12px;
  padding-bottom: 12px;
  margin-bottom: 4px;
}

.menu-toggle {
  background: none;
  border: 1px solid rgba(0, 0, 0, 0.12);
  border-radius: 8px;
  padding: 6px 12px;
  font-size: 20px;
  cursor: pointer;
  color: var(--text-main);
  line-height: 1;
}

.mobile-title {
  font-weight: 700;
  font-size: 16px;
  color: var(--text-main);
}

@media (max-width: 960px) {
  .page-shell {
    position: relative;
  }

  .sidebar {
    position: fixed;
    top: 0;
    left: 0;
    height: 100vh;
    width: 260px;
    min-width: 260px;
    transform: translateX(-100%);
    opacity: 0;
    box-shadow: 0 0 40px rgba(0, 0, 0, 0.3);
  }

  .sidebar-open {
    transform: translateX(0);
    opacity: 1;
  }

  .sidebar .brand-text {
    display: block;
  }

  .mobile-bar {
    display: flex;
  }

  .nav-list {
    flex-direction: column;
  }
}

@media (max-width: 600px) {
  .sidebar {
    width: 240px;
    min-width: 240px;
    padding: 16px;
  }

  .brand strong {
    font-size: 16px;
  }
}
</style>
