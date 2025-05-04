import {createRouter, createWebHashHistory} from 'vue-router'
import HomeView from '../views/HomeView.vue'
import LoginView from '../views/LoginView.vue'
import InterfaceView from '../views/InterfaceView.vue'

import {authStore} from '@/stores/auth'
import {securityStore} from '@/stores/security'
import {notify} from "@kyvg/vue3-notification";

const router = createRouter({
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
      path: '/interface',
      name: 'interface',
      component: InterfaceView
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
    }
  ],
  linkActiveClass: "active",
  linkExactActiveClass: "exact-active",
})

router.beforeEach(async (to) => {
  const auth = authStore()

  // check if the request was a successful oauth login
  if ('wgLoginState' in to.query && !auth.IsAuthenticated) {
    const state = to.query['wgLoginState']
    const returnUrl = auth.ReturnUrl
    console.log("Oauth login callback:", state)

    if (state === "success") {
      try {
        const uid = await auth.LoadSession()
        console.log("Oauth login completed for UID:", uid)
        console.log("Continuing to:", returnUrl)

        notify({
          title: "Logged in",
          text: "Authentication suceeded!",
          type: 'success',
        })

        auth.ResetReturnUrl()
        return returnUrl
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

  // redirect to login page if not logged in and trying to access a restricted page
  const publicPages = ['/', '/login', '/key-generator']
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
