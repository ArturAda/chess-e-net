const ChessApi = (() => {
    const API_BASE_URL = (window.CHESS_API_BASE_URL || '/api').replace(/\/$/, '');
    const TOKEN_STORAGE_KEY = 'chessemag_jwt';

    let memoryToken = '';

    class ApiError extends Error {
        constructor(message, { status = 0, payload = null } = {}) {
            super(message);
            this.name = 'ApiError';
            this.status = status;
            this.payload = payload;
        }
    }

    function getToken() {
        try {
            return localStorage.getItem(TOKEN_STORAGE_KEY) || memoryToken;
        } catch {
            return memoryToken;
        }
    }

    function setToken(token) {
        memoryToken = token || '';
        try {
            if (token) {
                localStorage.setItem(TOKEN_STORAGE_KEY, token);
            } else {
                localStorage.removeItem(TOKEN_STORAGE_KEY);
            }
        } catch {
            // Keep the token in memory when browser storage is unavailable.
        }
    }

    function clearToken() {
        setToken('');
    }

    function hasToken() {
        return Boolean(getToken());
    }

    async function request(path, { method = 'GET', body = null, auth = false } = {}) {
        const headers = {
            Accept: 'application/json'
        };

        const options = {
            method,
            headers
        };

        if (body !== null) {
            headers['Content-Type'] = 'application/json';
            options.body = JSON.stringify(body);
        }

        if (auth) {
            const token = getToken();
            if (!token) {
                throw new ApiError('Authorization token is missing.', { status: 401 });
            }
            headers.Authorization = `Bearer ${token}`;
        }

        let response;
        try {
            response = await fetch(`${API_BASE_URL}${path}`, options);
        } catch {
            throw new ApiError('Backend is unavailable. Check that localhost:8080 is running.');
        }

        const payload = await parseJSONResponse(response);
        if (!response.ok) {
            const message = payload?.error || payload?.message || `Request failed with status ${response.status}.`;
            throw new ApiError(message, { status: response.status, payload });
        }

        return payload;
    }

    async function parseJSONResponse(response) {
        const text = await response.text();
        if (!text) return null;

        try {
            return JSON.parse(text);
        } catch {
            return { message: text };
        }
    }

    async function register({ username, email, password }) {
        return request('/register', {
            method: 'POST',
            body: { username, email, password }
        });
    }

    async function login({ email, password }) {
        const payload = await request('/login', {
            method: 'POST',
            body: { email, password }
        });

        if (!payload?.token) {
            throw new ApiError('Backend did not return a JWT token.');
        }

        setToken(payload.token);
        return payload;
    }

    async function me() {
        return request('/me', { auth: true });
    }

    function logout() {
        clearToken();
    }

    function getErrorMessage(error) {
        if (error instanceof ApiError) {
            return error.message;
        }
        return error?.message || 'Unexpected frontend error.';
    }

    return {
        ApiError,
        register,
        login,
        me,
        logout,
        getToken,
        setToken,
        clearToken,
        hasToken,
        request,
        getErrorMessage
    };
})();

window.ChessApi = ChessApi;
