import {createRouter, createWebHashHistory} from 'vue-router'
import HomeView from '../views/HomeView.vue'
import LoginView from '../views/LoginView.vue'

import {authStore} from '@/stores/auth'
import {securityStore} from '@/stores/security'
import {notify} from "@kyvg/vue3-notification";

export const publicPages = ['/', '/login', '/key-generator', '/ip-calculator']

const router = createRouter({
  // No base argument: createWebHashHistory() defaults to location.pathname + location.search,
  // which is correct for /app/, {web.base_path}/app/ and the dev server at /.
  history: createWebHashHistory(),
  routes: [
    {
      path: '/',
      name: 'home',
      component: HomeView
    },
    {
      path: '/login',
      name: 'login',
      component: LoginView
    },
    {
      path: '/interfaces',
      name: 'interfaces',
      // route level code-splitting
      // this generates a separate chunk (About.[hash].js) for this route
      // which is lazy-loaded when the route is visited.
      component: () => import('../views/InterfaceView.vue')
    },
    {
      path: '/users',
      name: 'users',
      // route level code-splitting
      // this generates a separate chunk (About.[hash].js) for this route
      // which is lazy-loaded when the route is visited.
      component: () => import('../views/UserView.vue')
    },
    {
      path: '/profile',
      name: 'profile',
      // route level code-splitting
      // this generates a separate chunk (About.[hash].js) for this route
      // which is lazy-loaded when the route is visited.
      component: () => import('../views/ProfileView.vue')
    },
    {
      path: '/peer/config/:id',
      name: 'peer-config-download',
      // This is a "deep link" target used by link-only configuration emails. As it is not part of the
      // public pages, unauthenticated users are redirected to the login page first and are returned here
      // (starting the download) only after a successful authentication.
      component: () => import('../views/PeerConfigDownloadView.vue')
    },
    {
      path: '/settings',
      name: 'settings',
      // route level code-splitting
      // this generates a separate chunk (About.[hash].js) for this route
      // which is lazy-loaded when the route is visited.
      component: () => import('../views/SettingsView.vue')
    },
    {
      path: '/audit',
      name: 'audit',
      // route level code-splitting
      // this generates a separate chunk (About.[hash].js) for this route
      // which is lazy-loaded when the route is visited.
      component: () => import('../views/AuditView.vue')
    },
    {
      path: '/key-generator',
      name: 'key-generator',
      // route level code-splitting
      // this generates a separate chunk (About.[hash].js) for this route
      // which is lazy-loaded when the route is visited.
      component: () => import('../views/KeyGeneraterView.vue')
    },
    {
      path: '/ip-calculator',
      name: 'ip-calculator',
      // route level code-splitting
      // this generates a separate chunk (About.[hash].js) for this route
      // which is lazy-loaded when the route is visited.
      component: () => import('../views/IPCalculatorView.vue')
    }
  ],
  linkActiveClass: "active",
  linkExactActiveClass: "exact-active",
})

router.beforeEach(async (to) => {
  const auth = authStore()

  // check if the request was a successful oauth login
  const searchParams = new URLSearchParams(window.location.search)
  const oauthState = to.query['wgLoginState'] || searchParams.get('wgLoginState')

  if (oauthState && !auth.IsAuthenticated) {
    const returnUrl = auth.ReturnUrl
    console.log("Oauth login callback:", oauthState)

    if (oauthState === "success") {
      try {
        const uid = await auth.LoadSession()
        console.log("Oauth login completed for UID:", uid)
        console.log("Continuing to:", returnUrl)

        notify({
          title: "Logged in",
          text: "Authentication succeeded!",
          type: 'success',
        })

        auth.ResetReturnUrl()
        if (searchParams.has('wgLoginState')) {
          const cleanUrl = window.location.pathname + window.location.hash
          window.history.replaceState(null, '', cleanUrl)
        }
        return returnUrl || '/'
      } catch (e) {
        notify({
          title: "Login failed!",
          text: "Oauth session is invalid!",
          type: 'error',
        })

        return '/login'
      }
    } else {
      notify({
        title: "Login failed!",
        text: "Authentication via Oauth failed!",
        type: 'error',
      })

      return '/login'
    }
  }

  // ensure session validity is verified with backend before checking route access
  if (!auth.sessionChecked) {
    try {
      await auth.EnsureSession()
    } catch (e) {
      // session is not authenticated
    }
  }

  // redirect to returnUrl if already authenticated and accessing login page
  if (to.path === '/login' && auth.IsAuthenticated) {
    const returnUrl = auth.ReturnUrl
    if (returnUrl && returnUrl !== '/login') {
      auth.ResetReturnUrl()
      return returnUrl
    }
    return '/'
  }

  // redirect to login page if not logged in and trying to access a restricted page
  const authRequired = !publicPages.includes(to.path)

  if (authRequired && !auth.IsAuthenticated) {
    auth.SetReturnUrl(to.fullPath) // store the original destination before starting the auth process
    return '/login'
  }
})

router.afterEach(async (to, from) => {
  const sec = securityStore()
  const csrfPages = ['/', '/login']

  if (csrfPages.includes(to.path)) {
    await sec.LoadSecurityProperties() // make sure we have a valid csrf token
  }
})

export default router
