import { defineStore } from 'pinia'

export const useAppStore = defineStore('app', {
  state: () => ({
    apiBase: '/api',
  }),
})

