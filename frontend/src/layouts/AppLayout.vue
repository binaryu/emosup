<template>
  <div class="page-shell">
    <aside class="sidebar" :class="{ 'sidebar-open': sidebarOpen, 'is-collapsed': isCollapsed }">
      <div class="brand">
        <span class="brand-badge">EM</span>
        <div class="brand-text" v-show="!isCollapsed">
          <strong>Emos Upload</strong>
          <p>Task Manager</p>
        </div>
      </div>

      <nav class="nav-list">
        <RouterLink v-for="item in navItems" :key="item.to" :to="item.to" class="nav-link" @click="closeSidebar" :title="isCollapsed ? item.label : ''">
          <span class="nav-icon" v-html="item.icon"></span>
          <span class="nav-label" v-show="!isCollapsed">{{ item.label }}</span>
        </RouterLink>
      </nav>
      
      <div class="sidebar-footer">
        <div class="user-row" v-if="authStore.username" :title="authStore.username">
          <span class="nav-icon">
            <svg width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="M20 21v-2a4 4 0 0 0-4-4H8a4 4 0 0 0-4 4v2"></path><circle cx="12" cy="7" r="4"></circle></svg>
          </span>
          <span class="nav-label" v-show="!isCollapsed">{{ authStore.username }}</span>
        </div>

        <button class="action-btn" @click="toggleTheme" :title="isDark ? '切换亮色' : '切换暗色'">
          <span class="nav-icon">
            <svg v-if="isDark" width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><circle cx="12" cy="12" r="5"></circle><line x1="12" y1="1" x2="12" y2="3"></line><line x1="12" y1="21" x2="12" y2="23"></line><line x1="4.22" y1="4.22" x2="5.64" y2="5.64"></line><line x1="18.36" y1="18.36" x2="19.78" y2="19.78"></line><line x1="1" y1="12" x2="3" y2="12"></line><line x1="21" y1="12" x2="23" y2="12"></line><line x1="4.22" y1="19.78" x2="5.64" y2="18.36"></line><line x1="18.36" y1="5.64" x2="19.78" y2="4.22"></line></svg>
            <svg v-else width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="M21 12.79A9 9 0 1 1 11.21 3 7 7 0 0 0 21 12.79z"></path></svg>
          </span>
          <span class="nav-label" v-show="!isCollapsed">{{ isDark ? 'Light Mode' : 'Dark Mode' }}</span>
        </button>

        <button class="action-btn" @click="handleLogout" title="退出登录">
          <span class="nav-icon">
            <svg width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="M9 21H5a2 2 0 0 1-2-2V5a2 2 0 0 1 2-2h4"></path><polyline points="16 17 21 12 16 7"></polyline><line x1="21" y1="12" x2="9" y2="12"></line></svg>
          </span>
          <span class="nav-label" v-show="!isCollapsed">退出登录</span>
        </button>

        <button class="action-btn collapse-btn" @click="toggleCollapse" v-if="!isMobile" :title="isCollapsed ? '展开' : '收起'">
          <span class="nav-icon">
            <svg v-if="isCollapsed" width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><polyline points="13 17 18 12 13 7"></polyline><polyline points="6 17 11 12 6 7"></polyline></svg>
            <svg v-else width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><polyline points="11 17 6 12 11 7"></polyline><polyline points="18 17 13 12 18 7"></polyline></svg>
          </span>
          <span class="nav-label" v-show="!isCollapsed">收起侧边栏</span>
        </button>
      </div>
    </aside>

    <div v-if="sidebarOpen" class="sidebar-overlay" @click="closeSidebar" @touchmove.prevent></div>

    <main class="page-main">
      <div class="mobile-bar" v-if="isMobile">
        <button class="menu-toggle" @click="sidebarOpen = !sidebarOpen">
          <svg width="24" height="24" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><line x1="3" y1="12" x2="21" y2="12"></line><line x1="3" y1="6" x2="21" y2="6"></line><line x1="3" y1="18" x2="21" y2="18"></line></svg>
        </button>
        <span class="mobile-title">Emos Upload</span>
      </div>
      
      <div class="content-wrapper">
        <RouterView v-slot="{ Component }">
          <transition name="fade" mode="out-in">
            <component :is="Component" />
          </transition>
        </RouterView>
      </div>
    </main>
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted, onUnmounted } from 'vue'
import { useAuthStore } from '@/stores/auth'

const authStore = useAuthStore()

const navItems = [
  { label: '影片扫描', to: '/browse', icon: '<svg width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="M22 19a2 2 0 0 1-2 2H4a2 2 0 0 1-2-2V5a2 2 0 0 1 2-2h5l2 3h9a2 2 0 0 1 2 2z"></path></svg>' },
  { label: '扫描结果', to: '/scans', icon: '<svg width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><polyline points="22 12 18 12 15 21 9 3 6 12 2 12"></polyline></svg>' },
  { label: '任务队列', to: '/tasks', icon: '<svg width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><line x1="8" y1="6" x2="21" y2="6"></line><line x1="8" y1="12" x2="21" y2="12"></line><line x1="8" y1="18" x2="21" y2="18"></line><line x1="3" y1="6" x2="3.01" y2="6"></line><line x1="3" y1="12" x2="3.01" y2="12"></line><line x1="3" y1="18" x2="3.01" y2="18"></line></svg>' },
  { label: '系统配置', to: '/config', icon: '<svg width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><circle cx="12" cy="12" r="3"></circle><path d="M19.4 15a1.65 1.65 0 0 0 .33 1.82l.06.06a2 2 0 0 1 0 2.83 2 2 0 0 1-2.83 0l-.06-.06a1.65 1.65 0 0 0-1.82-.33 1.65 1.65 0 0 0-1 1.51V21a2 2 0 0 1-2 2 2 2 0 0 1-2-2v-.09A1.65 1.65 0 0 0 9 19.4a1.65 1.65 0 0 0-1.82.33l-.06.06a2 2 0 0 1-2.83 0 2 2 0 0 1 0-2.83l.06-.06a1.65 1.65 0 0 0 .33-1.82 1.65 1.65 0 0 0-1.51-1H3a2 2 0 0 1-2-2 2 2 0 0 1 2-2h.09A1.65 1.65 0 0 0 4.6 9a1.65 1.65 0 0 0-.33-1.82l-.06-.06a2 2 0 0 1 0-2.83 2 2 0 0 1 2.83 0l.06.06a1.65 1.65 0 0 0 1.82.33H9a1.65 1.65 0 0 0 1-1.51V3a2 2 0 0 1 2-2 2 2 0 0 1 2 2v.09a1.65 1.65 0 0 0 1 1.51 1.65 1.65 0 0 0 1.82-.33l.06-.06a2 2 0 0 1 2.83 0 2 2 0 0 1 0 2.83l-.06.06a1.65 1.65 0 0 0-.33 1.82V9a1.65 1.65 0 0 0 1.51 1H21a2 2 0 0 1 2 2 2 2 0 0 1-2 2h-.09a1.65 1.65 0 0 0-1.51 1z"></path></svg>' },
]

const sidebarOpen = ref(false)
const isCollapsed = ref(false)
const isMobile = ref(false)
const isDark = ref(false)

function checkMobile() {
  isMobile.value = window.innerWidth <= 960
  if (!isMobile.value) {
    sidebarOpen.value = false
  } else {
    isCollapsed.value = false // mobile sidebar is never collapsed
  }
}

function closeSidebar() {
  sidebarOpen.value = false
}

function toggleCollapse() {
  isCollapsed.value = !isCollapsed.value
}

function toggleTheme() {
  isDark.value = !isDark.value
  if (isDark.value) {
    document.documentElement.classList.add('dark')
    localStorage.setItem('theme', 'dark')
  } else {
    document.documentElement.classList.remove('dark')
    localStorage.setItem('theme', 'light')
  }
}

function handleLogout() {
  authStore.logout(true)
}

onMounted(() => {
  checkMobile()
  window.addEventListener('resize', checkMobile)
  if (!authStore.username) {
    authStore.fetchMe()
  }

  const savedTheme = localStorage.getItem('theme')
  if (savedTheme === 'dark' || (!savedTheme && window.matchMedia('(prefers-color-scheme: dark)').matches)) {
    isDark.value = true
    document.documentElement.classList.add('dark')
  }
})

onUnmounted(() => {
  window.removeEventListener('resize', checkMobile)
})
</script>

<style scoped>
.sidebar {
  width: 260px;
  min-width: 260px;
  padding: 24px 20px;
  background: var(--bg-sidebar);
  border-right: 1px solid var(--line-soft);
  display: flex;
  flex-direction: column;
  transition: all 0.3s cubic-bezier(0.4, 0, 0.2, 1);
  z-index: 20;
}

.sidebar.is-collapsed {
  width: 72px;
  min-width: 72px;
  padding: 24px 12px;
}

.brand {
  display: flex;
  align-items: center;
  gap: 12px;
  margin-bottom: 32px;
  padding: 0 4px;
  overflow: hidden;
  white-space: nowrap;
}

.brand strong {
  display: block;
  font-size: 18px;
  font-weight: 700;
  color: var(--text-main);
  letter-spacing: -0.02em;
}

.brand p {
  margin: 2px 0 0;
  color: var(--text-subtle);
  font-size: 13px;
}

.brand-badge {
  display: grid;
  place-items: center;
  width: 40px;
  height: 40px;
  border-radius: 10px;
  background: var(--brand-soft);
  color: var(--brand);
  font-weight: 700;
  font-size: 16px;
  flex-shrink: 0;
}

.nav-list {
  display: flex;
  flex-direction: column;
  gap: 8px;
  flex: 1;
}

.nav-link {
  padding: 12px 14px;
  border-radius: 10px;
  color: var(--text-subtle);
  font-weight: 500;
  font-size: 15px;
  transition: all 0.2s ease;
  display: flex;
  align-items: center;
  gap: 12px;
  text-decoration: none;
  overflow: hidden;
  white-space: nowrap;
}

.sidebar.is-collapsed .nav-link {
  justify-content: center;
  padding: 12px;
}

.nav-icon {
  display: flex;
  align-items: center;
  justify-content: center;
  flex-shrink: 0;
}

.nav-link.router-link-active {
  background: var(--brand-soft);
  color: var(--brand);
  font-weight: 600;
}

.nav-link:hover:not(.router-link-active) {
  background: var(--bg-hover);
  color: var(--text-main);
}

.sidebar-footer {
  margin-top: auto;
  padding-top: 16px;
  border-top: 1px solid var(--line-soft);
  display: flex;
  flex-direction: column;
  gap: 8px;
}

.user-row {
  display: flex;
  align-items: center;
  gap: 12px;
  padding: 8px 14px;
  color: var(--text-subtle);
  font-size: 13px;
  font-weight: 500;
  overflow: hidden;
  white-space: nowrap;
}

.sidebar.is-collapsed .user-row {
  justify-content: center;
  padding: 8px;
}

.user-row .nav-label {
  overflow: hidden;
  text-overflow: ellipsis;
}

.action-btn {
  width: 100%;
  padding: 12px 14px;
  border-radius: 10px;
  background: transparent;
  border: none;
  color: var(--text-subtle);
  cursor: pointer;
  font-weight: 500;
  font-size: 14px;
  transition: all 0.2s ease;
  display: flex;
  align-items: center;
  gap: 12px;
  overflow: hidden;
  white-space: nowrap;
}

.sidebar.is-collapsed .action-btn {
  justify-content: center;
  padding: 12px;
}

.action-btn:hover {
  background: var(--bg-hover);
  color: var(--text-main);
}

.sidebar-overlay {
  position: fixed;
  inset: 0;
  background: rgba(0, 0, 0, 0.4);
  z-index: 15;
  backdrop-filter: blur(2px);
  animation: fadeIn 0.3s ease;
}

@keyframes fadeIn {
  from { opacity: 0; }
  to { opacity: 1; }
}

.mobile-bar {
  display: none;
  align-items: center;
  gap: 16px;
  padding: 12px 16px;
  background: var(--bg-sidebar);
  border-bottom: 1px solid var(--line-soft);
  position: sticky;
  top: 0;
  z-index: 10;
  backdrop-filter: blur(8px);
  -webkit-backdrop-filter: blur(8px);
}

.menu-toggle {
  background: none;
  border: none;
  padding: 6px;
  cursor: pointer;
  color: var(--text-main);
  display: flex;
  align-items: center;
  justify-content: center;
  border-radius: 8px;
  transition: background 0.15s;
}
.menu-toggle:active { background: var(--bg-hover); }

.mobile-title {
  font-weight: 700;
  font-size: 18px;
  color: var(--text-main);
}

.content-wrapper {
  max-width: 1200px;
  margin: 0 auto;
}

/* Page Transitions */
.fade-enter-active,
.fade-leave-active {
  transition: opacity 0.25s ease, transform 0.25s ease;
}

.fade-enter-from {
  opacity: 0;
  transform: translateY(10px);
}

.fade-leave-to {
  opacity: 0;
  transform: translateY(-10px);
}

@media (max-width: 960px) {
  .page-shell {
    flex-direction: column;
  }

  .sidebar {
    position: fixed;
    top: 0;
    left: 0;
    height: 100vh;
    transform: translateX(-100%);
  }

  .sidebar-open {
    transform: translateX(0);
  }

  .mobile-bar {
    display: flex;
  }
  
  .page-main {
    padding: 0;
  }
  
  .content-wrapper {
    padding: 12px;
  }
}

@media (max-width: 480px) {
  .content-wrapper {
    padding: 8px;
  }
}
</style>
