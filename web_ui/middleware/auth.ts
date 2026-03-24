// middleware/auth.ts
import usePB from "@/composables/usePB";

export default defineNuxtRouteMiddleware((to) => {
  const config = useRuntimeConfig()
  const pb = usePB()
  const cookie = useCookie('pb_auth')

  pb.value.authStore.loadFromCookie(cookie.value || '')

  if (!pb.value.authStore.isValid && to.path !== '/login')
    return navigateTo('/login', { redirectCode: 401 })
})
