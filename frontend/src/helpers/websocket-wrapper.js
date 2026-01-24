import { peerStore } from '@/stores/peers';
import { interfaceStore } from '@/stores/interfaces';
import { authStore } from '@/stores/auth';

let socket = null;
let reconnectTimer = null;
let failureCount = 0;

export const websocketWrapper = {
    connect() {
        if (socket) {
            console.log('WebSocket already connected, re-using existing connection.');
            return;
        }

        const protocol = WGPORTAL_BACKEND_BASE_URL.startsWith('https://') ? 'wss://' : 'ws://';
        const baseUrl = WGPORTAL_BACKEND_BASE_URL.replace(/^https?:\/\//, '');
        const url = `${protocol}${baseUrl}/ws`;

        socket = new WebSocket(url);

        socket.onopen = () => {
            console.log('WebSocket connected');
            failureCount = 0;
            if (reconnectTimer) {
                clearInterval(reconnectTimer);
                reconnectTimer = null;
            }
        };

        socket.onclose = () => {
            console.log('WebSocket disconnected');
            failureCount++;
            socket = null;
            this.scheduleReconnect();
        };

        socket.onerror = (error) => {
            console.error('WebSocket error:', error);
            failureCount++;
            socket.close();
            socket = null;
        };

        socket.onmessage = (event) => {
            const message = JSON.parse(event.data);
            switch (message.type) {
                case 'peer_stats':
                    peerStore().updatePeerTrafficStats(message.data);
                    break;
                case 'interface_stats':
                    interfaceStore().updateInterfaceTrafficStats(message.data);
                    break;
            }
        };
    },

    disconnect() {
        if (socket) {
            socket.close();
            socket = null;
        }
        if (reconnectTimer) {
            clearInterval(reconnectTimer);
            reconnectTimer = null;
            failureCount = 0;
        }
    },

    scheduleReconnect() {
        if (reconnectTimer) return;
        if (!authStore().IsAuthenticated) return; // Don't reconnect if not logged in

        reconnectTimer = setInterval(() => {
            if (failureCount > 2) {
                console.log('WebSocket connection unavailable, giving up.');
                clearInterval(reconnectTimer);
                reconnectTimer = null;
                return;
            }

            console.log('Attempting to reconnect WebSocket...');
            this.connect();
        }, 5000);
    }
};
