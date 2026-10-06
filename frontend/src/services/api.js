import axios from 'axios'

// Defaults to a path relative to whatever origin served this page —
// this is what makes the app work correctly no matter which IP/port
// it's accessed from (localhost during development, or a LAN IP like
// 192.168.1.15:8080 during an offline exam). The Go backend serves
// both the API and this built frontend from the same origin, so a
// relative path always resolves correctly. Set VITE_API_BASE_URL only
// for an unusual deployment where the API lives on a different origin
// than the frontend.
const API_BASE_URL = import.meta.env.VITE_API_BASE_URL || '/api/v1'

export const apiClient = axios.create({
  baseURL: API_BASE_URL,
})

apiClient.interceptors.request.use((config) => {
  const token = localStorage.getItem('es_access_token')
  if (token) {
    config.headers.Authorization = `Bearer ${token}`
  }
  return config
})

let isRefreshing = false
let pendingQueue = []

apiClient.interceptors.response.use(
  (res) => res,
  async (error) => {
    const original = error.config
    if (error.response?.status === 401 && !original._retry) {
      original._retry = true
      const refreshToken = localStorage.getItem('es_refresh_token')
      if (!refreshToken) {
        authApi.clearSession()
        return Promise.reject(error)
      }
      if (isRefreshing) {
        return new Promise((resolve) => pendingQueue.push(() => resolve(apiClient(original))))
      }
      isRefreshing = true
      try {
        const { data } = await axios.post(`${API_BASE_URL}/auth/refresh`, { refresh_token: refreshToken })
        localStorage.setItem('es_access_token', data.data.access_token)
        pendingQueue.forEach((fn) => fn())
        pendingQueue = []
        return apiClient(original)
      } catch (e) {
        authApi.clearSession()
        return Promise.reject(e)
      } finally {
        isRefreshing = false
      }
    }
    return Promise.reject(error)
  }
)

// --- Auth ---
export const authApi = {
  login: (email, password) => apiClient.post('/auth/login', { email, password }).then((r) => r.data.data),
  logout: () => apiClient.post('/auth/logout'),
  register: (payload) => apiClient.post('/auth/register', payload).then((r) => r.data.data),
  saveSession: ({ access_token, refresh_token, user }) => {
    localStorage.setItem('es_access_token', access_token)
    localStorage.setItem('es_refresh_token', refresh_token)
    localStorage.setItem('es_user', JSON.stringify(user))
  },
  clearSession: () => {
    localStorage.removeItem('es_access_token')
    localStorage.removeItem('es_refresh_token')
    localStorage.removeItem('es_user')
  },
  currentUser: () => {
    const raw = localStorage.getItem('es_user')
    return raw ? JSON.parse(raw) : null
  },
}

// --- Users ---
export const userApi = {
  list: (role) => apiClient.get('/users', { params: role ? { role } : {} }).then((r) => r.data.data),
  suspend: (id) => apiClient.post(`/users/${id}/suspend`),
  reinstate: (id) => apiClient.post(`/users/${id}/reinstate`),
}

// --- Exams ---
export const examApi = {
  list: () => apiClient.get('/exams').then((r) => r.data.data),
  get: (id) => apiClient.get(`/exams/${id}`).then((r) => r.data.data),
  create: (payload) => apiClient.post('/exams', payload).then((r) => r.data.data),
  update: (id, payload) => apiClient.put(`/exams/${id}`, payload).then((r) => r.data.data),
  remove: (id) => apiClient.delete(`/exams/${id}`),
  addQuestion: (examId, payload) => apiClient.post(`/exams/${examId}/questions`, payload).then((r) => r.data.data),
  listQuestions: (examId) => apiClient.get(`/exams/${examId}/questions`).then((r) => r.data.data),
  deleteQuestion: (examId, questionId) => apiClient.delete(`/exams/${examId}/questions/${questionId}`),
  uploadQuestionsPDF: (examId, file) => {
    const formData = new FormData()
    formData.append('file', file)
    return apiClient
      .post(`/exams/${examId}/questions/upload-pdf`, formData, {
        headers: { 'Content-Type': 'multipart/form-data' },
      })
      .then((r) => r.data.data)
  },
  schedule: (id) => apiClient.post(`/exams/${id}/schedule`).then((r) => r.data.data),
  publish: (id) => apiClient.post(`/exams/${id}/publish`).then((r) => r.data.data),
  start: (id) => apiClient.post(`/exams/${id}/start`).then((r) => r.data.data),
  end: (id) => apiClient.post(`/exams/${id}/end`).then((r) => r.data.data),
  report: (id) => apiClient.get(`/exams/${id}/report`).then((r) => r.data.data),
  startAttempt: (id) => apiClient.post(`/exams/${id}/start-attempt`).then((r) => r.data.data),
  submitAttempt: (attemptId) => apiClient.post(`/exams/attempts/${attemptId}/submit`).then((r) => r.data.data),
  submitAnswer: (payload) => apiClient.post('/answers', payload).then((r) => r.data.data),
}

// --- Monitoring ---
export const monitoringApi = {
  listEvents: (examId) => apiClient.get(`/exams/${examId}/events`).then((r) => r.data.data),
  reviewEvent: (examId, eventId) => apiClient.post(`/exams/${examId}/events/${eventId}/review`),
  verifyFace: (payload) => apiClient.post('/face/verify', payload).then((r) => r.data.data),
  analyzeFrame: (payload) => apiClient.post('/monitoring/analyze-frame', payload).then((r) => r.data.data),
}

// --- Dashboard ---
export const dashboardApi = {
  summary: () => apiClient.get('/dashboard').then((r) => r.data.data),
}

export function openMonitoringSocket(examId) {
  const token = localStorage.getItem('es_access_token')

  // If API_BASE_URL is absolute (an explicit VITE_API_BASE_URL was
  // set), derive the WebSocket origin from it. Otherwise — the normal
  // case, matching the relative default above — derive it from the
  // page's own origin, so this also works correctly at whatever
  // LAN IP the exam server is reached at.
  let wsBase
  if (/^https?:\/\//.test(API_BASE_URL)) {
    wsBase = API_BASE_URL.replace(/^http/, 'ws').replace(/\/api\/v1\/?$/, '')
  } else {
    const protocol = window.location.protocol === 'https:' ? 'wss' : 'ws'
    wsBase = `${protocol}://${window.location.host}`
  }

  return new WebSocket(`${wsBase}/ws/exams/${examId}/monitor?token=${token}`)
}
