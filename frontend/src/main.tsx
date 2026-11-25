import { StrictMode } from 'react'
import { createRoot } from 'react-dom/client'
import { BrowserRouter } from 'react-router'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { Toaster } from 'sonner'
import { App } from './App'
import { ApiError } from './api/client'
import { AlertFeedProvider } from './context/AlertFeedContext'
import './styles.css'

const queryClient = new QueryClient({
  defaultOptions: {
    queries: {
      staleTime: 10_000,
      refetchOnWindowFocus: true,
      // Retry network hiccups, but not 4xx answers that won't change.
      retry: (count, err) => !(err instanceof ApiError && err.status >= 400 && err.status < 500) && count < 2,
    },
  },
})

createRoot(document.getElementById('root')!).render(
  <StrictMode>
    <QueryClientProvider client={queryClient}>
      <BrowserRouter>
        <AlertFeedProvider>
          <App />
          <Toaster position="top-right" richColors closeButton visibleToasts={3} offset={{ top: 64 }} mobileOffset={{ top: 64 }} />
        </AlertFeedProvider>
      </BrowserRouter>
    </QueryClientProvider>
  </StrictMode>,
)
