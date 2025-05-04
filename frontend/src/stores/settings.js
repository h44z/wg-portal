import { defineStore } from 'pinia'

import { notify } from "@kyvg/vue3-notification";
import { apiWrapper } from '@/helpers/fetch-wrapper'

const baseUrl = `/config`

export const settingsStore = defineStore('settings', {
    state: () => ({
        settings: {},
    }),
    getters: {
        Setting: (state) => {
            return (key) => (key in state.settings) ? state.settings[key] : undefined
        }
    },
    actions: {
        setSettings(settings) {
            this.settings = settings
        },
        // LoadSecurityProperties always returns a fulfilled promise, even if the request failed.
        async LoadSettings() {
            await apiWrapper.get(`${baseUrl}/settings`)
                .then(data => this.setSettings(data))
                .catch(error => {
                    this.setSettings({});
                    console.log("Failed to load settings: ", error);
                    notify({
                        title: "Backend Connection Failure",
                        text: "Failed to load settings!",
                    });
                })
        }
    }
});
