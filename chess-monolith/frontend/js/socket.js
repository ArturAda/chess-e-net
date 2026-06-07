const ChessSocket = (() => {
    const DEFAULT_WS_PATH = '/ws';
    const LOGIN_REQUIRED_MESSAGE = 'Log in before joining a match.';
    const CONNECTION_FAILED_MESSAGE = 'Could not enter the match room. Try again.';
    const CONNECTION_LOST_MESSAGE = 'Game connection was lost. Start search again.';
    const INVALID_SERVER_MESSAGE = 'The match room sent an unexpected update. Refresh the page if this repeats.';

    let socket = null;
    let connectPromise = null;
    let activeToken = '';
    const listeners = new Map();
    const suppressedCloseSockets = new WeakSet();

    function on(type, handler) {
        if (!listeners.has(type)) {
            listeners.set(type, new Set());
        }
        listeners.get(type).add(handler);
        return () => off(type, handler);
    }

    function off(type, handler) {
        listeners.get(type)?.delete(handler);
    }

    function emit(type, payload = null, message = null) {
        listeners.get(type)?.forEach(handler => handler(payload, message));
        listeners.get('*')?.forEach(handler => handler(type, payload, message));
    }

    function buildWebSocketURL(token) {
        const base = window.CHESS_WS_URL || DEFAULT_WS_PATH;
        const url = new URL(base, window.location.href);
        url.protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:';
        url.searchParams.set('token', token);
        return url.toString();
    }

    function isOpen() {
        return socket?.readyState === WebSocket.OPEN;
    }

    function isConnecting() {
        return socket?.readyState === WebSocket.CONNECTING;
    }

    function connect(token) {
        if (!token) {
            return Promise.reject(new Error(LOGIN_REQUIRED_MESSAGE));
        }

        if (isOpen() && activeToken === token) {
            return Promise.resolve(socket);
        }

        if (isConnecting() && connectPromise && activeToken === token) {
            return connectPromise;
        }

        close({ emitClose: false });
        activeToken = token;

        connectPromise = new Promise((resolve, reject) => {
            const nextSocket = new WebSocket(buildWebSocketURL(token));
            socket = nextSocket;

            nextSocket.addEventListener('open', () => {
                connectPromise = null;
                emit('OPEN');
                resolve(nextSocket);
            }, { once: true });

            nextSocket.addEventListener('message', event => {
                handleRawMessage(event.data);
            });

            nextSocket.addEventListener('error', () => {
                const error = new Error(CONNECTION_FAILED_MESSAGE);
                emit('ERROR', {
                    code: 'SOCKET_ERROR',
                    message: error.message,
                    recoverable: true
                });

                if (connectPromise) {
                    connectPromise = null;
                    reject(error);
                }
            }, { once: true });

            nextSocket.addEventListener('close', event => {
                if (suppressedCloseSockets.has(nextSocket)) {
                    suppressedCloseSockets.delete(nextSocket);
                    return;
                }

                const wasCurrentSocket = socket === nextSocket;
                if (wasCurrentSocket) {
                    socket = null;
                    activeToken = '';
                }
                connectPromise = null;
                emit('CLOSE', {
                    code: event.code,
                    reason: event.reason,
                    wasClean: event.wasClean
                });
            });
        });

        return connectPromise;
    }

    function handleRawMessage(raw) {
        let message;
        try {
            message = JSON.parse(raw);
        } catch {
            emit('ERROR', {
                code: 'INVALID_SERVER_MESSAGE',
                message: INVALID_SERVER_MESSAGE,
                recoverable: true
            });
            return;
        }

        if (!message?.type) {
            emit('ERROR', {
                code: 'INVALID_SERVER_MESSAGE',
                message: INVALID_SERVER_MESSAGE,
                recoverable: true
            });
            return;
        }

        emit(message.type, message.payload || null, message);
    }

    function send(type, payload = null) {
        if (!isOpen()) {
            throw new Error(CONNECTION_LOST_MESSAGE);
        }

        const message = { type };
        if (payload !== null && payload !== undefined) {
            message.payload = payload;
        }

        socket.send(JSON.stringify(message));
    }

    function joinQueue({ mode, boardSize, isRanked = false, timeControlMinutes, visualState = null }) {
        const payload = {
            mode,
            board_size: boardSize,
            is_ranked: isRanked,
            time_limit: timeControlMinutes
        };

        if (visualState && typeof visualState === 'object') {
            payload.visual_state = visualState;
        }

        send('JOIN_QUEUE', payload);
    }

    function cancelQueue() {
        if (!isOpen()) return;
        send('CANCEL_QUEUE');
    }

    function move({ from, to, promotion = '' }) {
        const payload = { from, to };
        if (promotion) {
            payload.promotion = promotion;
        }
        send('MOVE', payload);
    }

    function leaveGame() {
        if (!isOpen()) return;
        send('LEAVE_GAME');
    }

    function offerDraw() {
        send('DRAW_OFFER');
    }

    function acceptDraw() {
        send('DRAW_ACCEPT');
    }

    function declineDraw() {
        send('DRAW_DECLINE');
    }

    function chatSticker(stickerId) {
        send('CHAT_STICKER', { sticker_id: stickerId });
    }

    function close({ emitClose = true } = {}) {
        if (!socket) return;

        const currentSocket = socket;
        if (!emitClose) {
            suppressedCloseSockets.add(currentSocket);
        }

        socket = null;
        activeToken = '';
        connectPromise = null;

        currentSocket.close();
    }

    return {
        on,
        off,
        connect,
        send,
        joinQueue,
        cancelQueue,
        move,
        leaveGame,
        offerDraw,
        acceptDraw,
        declineDraw,
        chatSticker,
        close,
        isOpen
    };
})();

window.ChessSocket = ChessSocket;
