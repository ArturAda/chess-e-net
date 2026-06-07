const ChessApi = (() => {
    const API_BASE_URL = (window.CHESS_API_BASE_URL || '/api').replace(/\/$/, '');
    const TOKEN_STORAGE_KEY = 'chessemag_jwt';
    const CONNECTION_MESSAGE = 'The game room is not responding. Try again in a moment.';
    const SESSION_MESSAGE = 'Your session expired. Log in again.';
    const GENERIC_MESSAGE = 'Something went wrong. Try again.';
    const TECHNICAL_MESSAGE_PATTERN = /\b(backend|localhost|websocket|jwt|authorization bearer|token|request failed|frontend|internal server error|invalid data|key:|binding)\b/i;

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
                throw new ApiError(SESSION_MESSAGE, { status: 401 });
            }
            headers.Authorization = `Bearer ${token}`;
        }

        let response;
        try {
            response = await fetch(`${API_BASE_URL}${path}`, options);
        } catch {
            throw new ApiError(CONNECTION_MESSAGE);
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
            throw new ApiError('Login did not finish. Try again.');
        }

        setToken(payload.token);
        return payload;
    }

    async function me() {
        return request('/me', { auth: true });
    }

    async function listGames() {
        return request('/games', { auth: true });
    }

    async function getGame(id) {
        if (!id) {
            throw new ApiError('Game id is required.');
        }
        return request(`/games/${encodeURIComponent(id)}`, { auth: true });
    }

    async function myRatings() {
        return request('/me/ratings', { auth: true });
    }

    async function leaderboard({ mode = 'classic', boardSize = 8, timeLimitMinutes = 10, limit = 50 } = {}) {
        const params = new URLSearchParams({
            mode,
            board_size: String(boardSize),
            time_limit: String(timeLimitMinutes),
            limit: String(limit)
        });
        return request(`/leaderboard?${params.toString()}`);
    }

    function logout() {
        clearToken();
    }

    function getErrorMessage(error) {
        if (error instanceof ApiError) {
            return playerFacingApiMessage(error);
        }
        return playerFacingText(error?.message, GENERIC_MESSAGE);
    }

    function playerFacingApiMessage(error) {
        const message = String(error?.message || '').trim();

        if (error?.status === 0) return CONNECTION_MESSAGE;
        if (error?.status === 401) {
            if (/invalid email or password|credentials/i.test(message)) {
                return 'Incorrect email or password.';
            }
            return SESSION_MESSAGE;
        }
        if (error?.status === 403) return 'This action is not available for your account.';
        if (error?.status === 404) return 'The requested game was not found.';
        if (error?.status >= 500) return 'The game room is busy right now. Try again soon.';
        if (error?.status === 400 && TECHNICAL_MESSAGE_PATTERN.test(message)) {
            return 'Check the entered fields and try again.';
        }

        return playerFacingText(message, GENERIC_MESSAGE);
    }

    function playerFacingText(message, fallback) {
        const text = String(message || '').trim();
        if (!text || TECHNICAL_MESSAGE_PATTERN.test(text)) {
            return fallback;
        }
        return text;
    }

    return {
        ApiError,
        register,
        login,
        me,
        listGames,
        getGame,
        myRatings,
        leaderboard,
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
