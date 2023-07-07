import { authStore } from '../stores/auth';
import { securityStore } from '../stores/security';

export const fetchWrapper = {
    url: apiUrl(),
    get: request('GET'),
    post: request('POST'),
    put: request('PUT'),
    delete: request('DELETE')
};

export const apiWrapper = {
    url: apiUrl(),
    get: apiRequest('GET'),
    post: apiRequest('POST'),
    put: apiRequest('PUT'),
    delete: apiRequest('DELETE')
};

// request can be used to query arbitrary URLs
function request(method) {
    return (url, body = undefined) => {
        const requestOptions = {
            method,
            headers: getHeaders(url)
        };
        if (body) {
            requestOptions.headers['Content-Type'] = 'application/json';
            requestOptions.body = JSON.stringify(body);
        }
        return fetch(url, requestOptions).then(handleResponse);
    }
}

// apiRequest uses WGPORTAL_BACKEND_BASE_URL as base URL
function apiRequest(method) {
    return (path, body = undefined) => {
        const url = WGPORTAL_BACKEND_BASE_URL + path
        const requestOptions = {
            method,
            headers: getHeaders(method, url)
        };
        if (body) {
            requestOptions.headers['Content-Type'] = 'application/json';
            requestOptions.body = JSON.stringify(body);
        }
        return fetch(url, requestOptions).then(handleResponse);
    }
}

// apiUrl uses WGPORTAL_BACKEND_BASE_URL as base URL
function apiUrl() {
    return (path) => {
        return WGPORTAL_BACKEND_BASE_URL + path
    }
}

// helper functions

function getHeaders(method, url) {
    // return auth header with jwt if user is logged in and request is to the api url
    const auth = authStore();
    const sec = securityStore();
    const isApiUrl = url.startsWith(WGPORTAL_BACKEND_BASE_URL);

    let headers = {};
    if (isApiUrl && ['POST', 'PUT', 'PATCH', 'DELETE'].includes(method)) {
        headers["X-CSRF-TOKEN"] = sec.CsrfToken;
    }
    if (isApiUrl && auth.IsAuthenticated) {
        headers["X-FRONTEND-UID"] = auth.UserIdentifier;
    }

    return headers;
}

function handleResponse(response) {
    return response.text().then(text => {
        const data = text && JSON.parse(text);

        if (!response.ok) {
            const auth = authStore();
            if ([401, 403].includes(response.status) && auth.IsAuthenticated) {
                console.log("automatic logout initiated...");
                // auto logout if 401 Unauthorized or 403 Forbidden response returned from api
                auth.Logout();
            }

            const error = (data && data.Message) || response.statusText;
            return Promise.reject(error);
        }

        return data;
    });
}