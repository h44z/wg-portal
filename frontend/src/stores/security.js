import { defineStore } from 'pinia'

import { notify } from "@kyvg/vue3-notification";
import { apiWrapper } from '@/helpers/fetch-wrapper'

export const securityStore = defineStore({
    id: 'security',
    state: () => ({
        csrfToken: "",
    }),
    getters: {
        CsrfToken: (state) => state.csrfToken,
    },
    actions: {
        SetCsrfToken(token) {
            this.csrfToken = token
        },
        // LoadSecurityProperties always returns a fulfilled promise, even if the request failed.
        async LoadSecurityProperties() {
            await apiWrapper.get(`/csrf`)
                .then(token => this.SetCsrfToken(token))
                .catch(error => {
                    this.SetCsrfToken("");
                    console.log("Failed to load csrf token: ", error);
                    notify({
                        title: "Backend Connection Failure",
                        text: "Failed to load csrf token!",
                    });
                })
        }
    }
});