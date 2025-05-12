import { defineStore } from 'pinia'

import { notify } from "@kyvg/vue3-notification";
import { apiWrapper } from '@/helpers/fetch-wrapper'
import router from '../router'
import { browserSupportsWebAuthn,startRegistration,startAuthentication } from '@simplewebauthn/browser';
import {base64_url_encode} from "@/helpers/encoding";

export const authStore = defineStore('auth',{
    state: () => ({
        // initialize state from local storage to enable user to stay logged in
        user: JSON.parse(localStorage.getItem('user')),
        providers: [],
        returnUrl: localStorage.getItem('returnUrl'),
        webAuthnCredentials: [],
        fetching: false,
    }),
    getters: {
        UserIdentifier: (state) => state.user?.Identifier || 'unknown',
        User: (state) => state.user,
        LoginProviders: (state) => state.providers,
        IsAuthenticated: (state) => state.user != null,
        IsAdmin: (state) => state.user?.IsAdmin || false,
        ReturnUrl: (state) => state.returnUrl || '/',
        IsWebAuthnEnabled: (state) => {
            if (state.webAuthnCredentials) {
                return state.webAuthnCredentials.length > 0
            }
            return false
        },
        WebAuthnCredentials: (state) => state.webAuthnCredentials || [],
        isFetching: (state) => state.fetching,
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
        // LoadWebAuthnCredentials returns promise that might have been rejected if the session was not authenticated.
        async LoadWebAuthnCredentials() {
            this.fetching = true
            return apiWrapper.get(`/auth/webauthn/credentials`)
                .then(credentials => {
                    this.setWebAuthnCredentials(credentials)
                })
                .catch(error => {
                    this.setWebAuthnCredentials([])
                    console.log("Failed to load webauthn credentials:", error)
                    notify({
                        title: "Backend Connection Failure",
                        text: error,
                        type: 'error',
                    })
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
        async RegisterWebAuthn() {
            // check if the browser supports WebAuthn
            if (!browserSupportsWebAuthn()) {
                console.error("WebAuthn is not supported by this browser.");
                notify({
                    title: "WebAuthn not supported",
                    text: "This browser does not support WebAuthn.",
                    type: 'error'
                });
                return Promise.reject(new Error("WebAuthn not supported"));
            }

            this.fetching = true
            console.log("Starting WebAuthn registration...")
            await apiWrapper.post(`/auth/webauthn/register/start`, {})
                .then(optionsJSON => {
                    notify({
                        title: "Passkey registration",
                        text: "Starting passkey registration, follow the instructions in the browser."
                    });
                    console.log("Started WebAuthn registration with options: ", optionsJSON)

                    return startRegistration({ optionsJSON: optionsJSON.publicKey }).then(attResp => {
                        console.log("Finishing WebAuthn registration...")
                        return apiWrapper.post(`/auth/webauthn/register/finish`, attResp)
                            .then(credentials =>  {
                                console.log("Passkey registration finished successfully: ", credentials)
                                this.setWebAuthnCredentials(credentials)
                                notify({
                                    title: "Passkey registration",
                                    text: "A new passkey has been registered successfully!",
                                    type: 'success'
                                });
                            })
                            .catch(err => {
                                this.fetching = false
                                console.error("Failed to register passkey:", err);
                                notify({
                                    title: "Passkey registration failed",
                                    text: err,
                                    type: 'error'
                                });
                            })
                    }).catch(err => {
                        this.fetching = false
                        console.error("Failed to start WebAuthn registration:", err);
                        notify({
                            title: "Failed to start Passkey registration",
                            text: err,
                            type: 'error'
                        });
                    })
                })
                .catch(err => {
                    this.fetching = false
                    console.error("Failed to start WebAuthn registration:", err);
                    notify({
                        title: "Failed to start WebAuthn registration",
                        text: err,
                        type: 'error'
                    });
                })
        },
        async DeleteWebAuthnCredential(credentialId) {
            this.fetching = true
            return apiWrapper.delete(`/auth/webauthn/credential/${base64_url_encode(credentialId)}`)
                .then(credentials =>  {
                    this.setWebAuthnCredentials(credentials)
                    notify({
                        title: "Success",
                        text: "Passkey deleted successfully!",
                        type: 'success',
                    })
                })
                .catch(err => {
                    this.fetching = false
                    console.error("Failed to delete webauthn credential:", err);
                    notify({
                        title: "Backend Connection Failure",
                        text: err,
                        type: 'error',
                    })
                })
        },
        async RenameWebAuthnCredential(credential) {
            this.fetching = true
            return apiWrapper.put(`/auth/webauthn/credential/${base64_url_encode(credential.ID)}`, {
                Name: credential.Name,
            })
                .then(credentials =>  {
                    this.setWebAuthnCredentials(credentials)
                    notify({
                        title: "Success",
                        text: "Passkey renamed successfully!",
                        type: 'success',
                    })
                })
                .catch(err => {
                    this.fetching = false
                    console.error("Failed to rename webauthn credential", credential.ID, ":", err);
                    notify({
                        title: "Backend Connection Failure",
                        text: err,
                        type: 'error',
                    })
                })
        },
        async LoginWebAuthn() {
            // check if the browser supports WebAuthn
            if (!browserSupportsWebAuthn()) {
                console.error("WebAuthn is not supported by this browser.");
                notify({
                    title: "WebAuthn not supported",
                    text: "This browser does not support WebAuthn.",
                    type: 'error'
                });
                return Promise.reject(new Error("WebAuthn not supported"));
            }

            this.fetching = true
            console.log("Starting WebAuthn login...")
            await apiWrapper.post(`/auth/webauthn/login/start`, {})
                .then(optionsJSON => {
                    console.log("Started WebAuthn login with options: ", optionsJSON)

                    return startAuthentication({ optionsJSON: optionsJSON.publicKey }).then(asseResp => {
                        console.log("Finishing WebAuthn login ...")
                        return apiWrapper.post(`/auth/webauthn/login/finish`, asseResp)
                            .then(user =>  {
                                console.log("Passkey login finished successfully for user:", user.Identifier)
                                this.ResetReturnUrl()
                                this.setUserInfo(user)
                                return user.Identifier
                            })
                            .catch(err => {
                                console.error("Failed to login with passkey:", err)
                                this.setUserInfo(null)
                                return Promise.reject(new Error("login failed"))
                            })
                    }).catch(err => {
                        console.error("Failed to finish passkey login:", err)
                        this.setUserInfo(null)
                        return Promise.reject(new Error("login failed"))
                    })
                })
                .catch(err => {
                    console.error("Failed to start passkey login:", err)
                    this.setUserInfo(null)
                    return Promise.reject(new Error("login failed"))
                })
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
        setWebAuthnCredentials(credentials) {
            this.fetching = false
            this.webAuthnCredentials = credentials
        }
    }
});
