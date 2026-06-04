import { defineStore } from 'pinia'
import { ref } from 'vue'
import { getAppState, setAppState } from '../db/browserDb'
import { DEFAULT_CENTRAL_SYSTEM_URL } from '../config/defaults'

export const useSettingsStore = defineStore('settings', () => {
  const centralSystemUrl = ref(DEFAULT_CENTRAL_SYSTEM_URL)
  const loading = ref(false)
  const error = ref<string | null>(null)

  async function loadSettings() {
    loading.value = true
    error.value = null
    try {
      centralSystemUrl.value =
        (await getAppState<string>('centralSystemUrl')) || DEFAULT_CENTRAL_SYSTEM_URL
    } catch (err: any) {
      error.value = err.message
    } finally {
      loading.value = false
    }
  }

  async function saveCentralSystemUrl(value: string) {
    const next = value.trim() || DEFAULT_CENTRAL_SYSTEM_URL
    centralSystemUrl.value = next
    await setAppState('centralSystemUrl', next)
  }

  return {
    centralSystemUrl,
    loading,
    error,
    loadSettings,
    saveCentralSystemUrl,
  }
})
