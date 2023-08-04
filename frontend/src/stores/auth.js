import { defineStore } from 'pinia'

import { notify } from "@kyvg/vue3-notification";
import { apiWrapper } from '@/helpers/fetch-wrapper'
import router from '../router'

export const authStore = defineStore({
    id: 'auth',
    state: () => ({
        // initialize state from local storage to enable user to stay logged in
        user: JSON.parse(localStorage.getItem('user')),
        providers: [],
        returnUrl: localStorage.getItem('returnUrl')
    }),
    getters: {
        UserIdentifier: (state) => state.user?.Identifier || 'unknown',
        User: (state) => state.user,
        LoginProviders: (state) => state.providers,
        IsAuthenticated: (state) => state.user != null,
        IsAdmin: (state) => state.user?.IsAdmin || false,
        ReturnUrl: (state) => state.returnUrl || '/',
    },
    actions: {
        SetReturnUrl(link) {
            this.returnUrl = link
            localStorage.setItem('returnUrl', link)
        },
        ResetReturnUrl() {
            this.returnUrl = null
            localStorage.removeItem('returnUrl')
        },
        // LoadProviders always returns a fulfilled promise, even if the request failed.
        async LoadProviders() {
            apiWrapper.get(`/auth/providers`)
                .then(providers => this.providers = providers)
                .catch(error => {
                    this.providers = []
                    console.log("Failed to load auth providers: ", error)
                    notify({
                        title: "Backend Connection Failure",
                        text: "Failed to load external authentication providers!",
                    })
                })
        },

        // LoadSession returns promise that might have been rejected if the session was not authenticated.
        async LoadSession() {
            return apiWrapper.get(`/auth/session`)
                .then(session => {
                    if (session.LoggedIn === true) {
                        this.ResetReturnUrl()
                        this.setUserInfo(session)
                        return session.UserIdentifier
                    } else {
                        this.setUserInfo(null)
                        return Promise.reject(new Error('session not authenticated'))
                    }
                })
                .catch(err => {
                    this.setUserInfo(null)
                    return Promise.reject(err)
                })
        },
        // Login returns promise that might have been rejected if the login attempt was not successful.
        async Login(username, password) {
            return apiWrapper.post(`/auth/login`, { username, password })
                .then(user =>  {
                    this.ResetReturnUrl()
                    this.setUserInfo(user)
                    return user.Identifier
                })
                .catch(err => {
                    console.log("Login failed:", err)
                    this.setUserInfo(null)
                    return Promise.reject(new Error("login failed"))
                })
        },
        async Logout() {
            this.setUserInfo(null)
            this.ResetReturnUrl() // just to be sure^^

            try {
                await apiWrapper.post(`/auth/logout`)
            } catch (e) {
                console.log("Logout request failed:", e)
            }

            notify({
                title: "Logged Out",
                text: "Logout successful!",
                type: "warn",
            })


            await router.push('/login')
        },
        // -- internal setters
        setUserInfo(userInfo) {
            // store user details and jwt in local storage to keep user logged in between page refreshes
            if (userInfo) {
                if ('UserIdentifier' in userInfo) { // session object
                    this.user = {
                        Identifier: userInfo['UserIdentifier'],
                        Firstname: userInfo['UserFirstname'],
                        Lastname: userInfo['UserLastname'],
                        Email: userInfo['UserEmail'],
                        IsAdmin: userInfo['IsAdmin']
                    }
                } else { // user object
                    this.user = {
                        Identifier: userInfo['Identifier'],
                        Firstname: userInfo['Firstname'],
                        Lastname: userInfo['Lastname'],
                        Email: userInfo['Email'],
                        IsAdmin: userInfo['IsAdmin']
                    }
                }
                localStorage.setItem('user', JSON.stringify(this.user))
            } else {
                this.user = null
                localStorage.removeItem('user')
            }
        },
    }
});