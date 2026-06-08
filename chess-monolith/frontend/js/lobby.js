let historyRecords = [];
const LEGACY_HISTORY_STORAGE_KEYS = [
    'chessemag_history',
    'chessemagHistory',
    'chessemag-game-history',
    'historyRecords'
];

let board = null;
let currentVisualBoardSize = null;
let currentTimeControlMinutes = null;
let currentGameMode = 'classic';
let currentCustomPosition = null;
let selectedCustomSquare = null;
let customDragState = null;
let customDragSuppressClickUntil = 0;
let selectedClassicBoardSize = null;
let selectedClassicTimeMinutes = null;
let selectedClassicIsRanked = false;
let selectedModernBoardSize = null;
let selectedModernTimeMinutes = null;
let selectedModernIsRanked = false;
let capturedByMe = [];
let capturedByOpponent = [];
let activeMatchRequest = null;
let queuedForMatch = false;
let activeRemoteGame = false;
let currentGameState = null;
let currentPlayerColor = null;
let currentGameId = null;
let currentIsRanked = false;
let currentValidMoves = {};
let pendingClassicMove = null;
let pendingPromotionMove = null;
let classicSnapbackInProgress = false;
let queuedClassicPositionUpdate = null;
let activeDrawOffer = null;
let drawOfferIntervalId = null;
let networkWarning = null;
let networkWarningIntervalId = null;
let ratingsState = {
    boardSize: 8,
    timeControlMinutes: 10,
    loading: false,
    error: '',
    leaderboard: null
};
let historySortDirection = 'desc';
let historyFilters = new Set();
let historyLoaded = false;
let historyLoading = false;
let historyLoadError = '';
let activeHistoryDetailRequest = 0;
let historyReplayState = null;
let historyReplayBoard = null;
let timerState = null;
let timerIntervalId = null;
let matchNotFoundTimeoutId = null;
let gameFinishedOverlayDelayId = null;
let gameFinishedRedirectTimeoutId = null;
let settingsGalleryRendered = false;
let emojiChatRendered = false;
let emojiMessages = [];
let userStyles = loadUserStyles();
let settings = loadCurrentSettings();
let accountProfile = loadAccountProfile();
let accountEditing = false;
let accountAuthMode = 'login';
let pendingVerificationEmail = '';
let accountVerificationDeadlineMs = 0;
let accountVerificationTimerId = null;
let securityConfig = {
    turnstile: {
        enabled: false,
        siteKey: ''
    }
};
let turnstileScriptPromise = null;
let turnstileWidgetId = null;
let turnstileToken = '';

document.addEventListener('DOMContentLoaded', () => {
    clearLegacyAccountProfile();
    clearLocalHistoryRecords();
    normalizeSettings();
    bindClassicSetupControls();
    bindModernSetupControls();
    bindHistoryControls();
    bindGameActionControls();
    bindRatingControls();
    bindSocketEvents();
    bindAccountForm();
    renderAccountProfile();
    loadSecurityConfig();
    refreshAccountFromBackend();
    setAccountEntryVisibility('page-menu');
    setPageScrollMode('page-menu');
    applySelectedBackground();
    applySelectedBoardSquares();
    initAmbientBackground();
    renderHistoryList();
});

window.addEventListener('resize', () => {
    markViewportResizing();
    if (board) {
        board.resize();
        paintRenderedClassicSquares();
    }
    if (historyReplayBoard) {
        historyReplayBoard.resize();
        paintRenderedClassicSquares('#history-replay-board', historyReplayState?.record?.visualState);
    }
});

function navigateTo(pageId) {
    const leavingClassic = document.getElementById('page-classic')?.classList.contains('active') && pageId !== 'page-classic';
    if (leavingClassic) {
        resetClassicEntry();
    }
    const leavingHistoryDetail = document.getElementById('page-history-detail')?.classList.contains('active') && pageId !== 'page-history-detail';
    if (leavingHistoryDetail) {
        destroyHistoryReplayBoard();
    }

    document.querySelectorAll('.page').forEach(page => {
        page.classList.remove('active');
    });

    const targetPage = document.getElementById(pageId);
    if (targetPage) {
        targetPage.classList.add('active');
    }

    setAccountEntryVisibility(pageId);
    setPageScrollMode(pageId);

    if (pageId === 'page-classic') {
        resetClassicEntry();
    }

    if (pageId === 'page-modern') {
        resetModernSetup();
    }

    if (pageId === 'page-history') {
        loadHistoryList({ force: true });
    }

    if (pageId === 'page-rating') {
        loadRatingPage({ force: true });
    }

    if (pageId === 'page-settings') {
        renderSettingsGallery();
    }

    if (pageId === 'page-account') {
        accountEditing = false;
        renderAccountProfile();
        showAccountMessage('');
    }
}

const accordions = document.querySelectorAll('.accordion-btn');
accordions.forEach(btn => {
    btn.addEventListener('click', function() {
        this.classList.toggle('active');
        const content = this.nextElementSibling;
        content.style.maxHeight = content.style.maxHeight ? null : `${content.scrollHeight}px`;
    });
});

function setAccountEntryVisibility(pageId) {
    document.getElementById('account-chip')?.classList.toggle('hidden', pageId !== 'page-menu');
}

function setPageScrollMode(pageId) {
    document.body.classList.toggle('no-page-scroll', pageId === 'page-menu' || pageId === 'page-settings' || pageId === 'page-classic');
    document.body.classList.toggle('settings-page-active', pageId === 'page-settings');
    applyFallingPiecesPreference();
}

function markViewportResizing() {
    document.body.classList.add('viewport-resizing');
    window.clearTimeout(viewportResizeTimeoutId);
    viewportResizeTimeoutId = window.setTimeout(() => {
        document.body.classList.remove('viewport-resizing');
    }, 180);
}

function initAmbientBackground() {
    renderAmbientBoardLayer();
    window.addEventListener('resize', () => {
        markViewportResizing();
        window.clearTimeout(ambientResizeTimeoutId);
        ambientResizeTimeoutId = window.setTimeout(renderAmbientBoardLayer, 120);
    });
    document.addEventListener('visibilitychange', applyFallingPiecesPreference);

    applyFallingPiecesPreference();
}

function renderAmbientBoardLayer() {
    const layer = document.getElementById('ambient-board-layer');
    if (!layer) return;

    const tileSize = window.innerWidth < 760 ? 112 : 160;
    const columns = Math.ceil(window.innerWidth / tileSize) + 4;
    const rows = Math.ceil(window.innerHeight / tileSize) + 4;
    const total = columns * rows;
    const signature = `${tileSize}:${columns}:${rows}`;
    if (ambientGridSignature === signature && layer.childElementCount === total) return;

    const squareStrategies = AMBIENT_SQUARE_IDS.map(id => getSquareStrategy(id));
    const fragment = document.createDocumentFragment();

    layer.style.setProperty('--ambient-square-size', `${tileSize}px`);
    layer.style.gridTemplateColumns = `repeat(${columns}, var(--ambient-square-size))`;

    for (let index = 0; index < total; index += 1) {
        const row = Math.floor(index / columns);
        const col = index % columns;
        const strategy = squareStrategies[(row + col * 2) % squareStrategies.length];
        const square = document.createElement('span');
        square.className = 'ambient-square';
        square.style.backgroundColor = strategy.getColor();
        square.style.backgroundImage = `url("${strategy.getSrc()}")`;
        fragment.appendChild(square);
    }

    layer.replaceChildren(fragment);
    ambientGridSignature = signature;
}

function startFallingPieces() {
    const layer = document.getElementById('falling-pieces-layer');
    if (!layer || ambientPieceIntervalId || document.hidden) return;

    for (let index = 0; index < FALLING_PIECE_INITIAL_COUNT; index += 1) {
        const timeoutId = window.setTimeout(() => spawnFallingPiece(layer), index * 520);
        ambientPieceTimeoutIds.push(timeoutId);
    }
    ambientPieceIntervalId = window.setInterval(() => spawnFallingPiece(layer), FALLING_PIECE_INTERVAL_MS);
}

function stopFallingPieces() {
    ambientPieceTimeoutIds.forEach(timeoutId => window.clearTimeout(timeoutId));
    ambientPieceTimeoutIds = [];
    window.clearInterval(ambientPieceIntervalId);
    ambientPieceIntervalId = null;
    const layer = document.getElementById('falling-pieces-layer');
    if (layer) {
        layer.innerHTML = '';
    }
}

function applyFallingPiecesPreference() {
    const reduceMotion = window.matchMedia('(prefers-reduced-motion: reduce)').matches;
    const saveData = Boolean(navigator.connection?.saveData);
    const isSettingsPage = document.body.classList.contains('settings-page-active');
    if (settings.fallingPiecesEnabled && !reduceMotion && !saveData && !isSettingsPage && !document.hidden) {
        startFallingPieces();
        return;
    }
    stopFallingPieces();
}

function spawnFallingPiece(layer) {
    if (!settings.fallingPiecesEnabled) return;
    if (document.body.classList.contains('settings-page-active')) return;
    if (document.hidden) return;
    if (layer.childElementCount >= FALLING_PIECE_MAX_COUNT) return;

    const pieceColor = Math.random() > 0.45 ? 'w' : 'b';
    const pieceType = PIECE_TYPES[Math.floor(Math.random() * PIECE_TYPES.length)];
    const piece = document.createElement('img');
    piece.className = 'falling-piece';
    piece.src = getPieceSrc(`${pieceColor}${pieceType}`);
    piece.alt = '';
    piece.style.setProperty('--fall-size', `${Math.round(34 + Math.random() * 30)}px`);
    piece.style.setProperty('--fall-x', `${Math.round(Math.random() * 100)}vw`);
    piece.style.setProperty('--fall-drift', `${Math.round(-80 + Math.random() * 160)}px`);
    piece.style.setProperty('--fall-duration', `${Math.round(13 + Math.random() * 11)}s`);
    piece.style.setProperty('--fall-opacity', `${(0.34 + Math.random() * 0.24).toFixed(2)}`);
    piece.style.setProperty('--fall-rotate-start', `${Math.round(-24 + Math.random() * 48)}deg`);
    piece.style.setProperty('--fall-rotate-end', `${Math.round(160 + Math.random() * 220)}deg`);
    piece.addEventListener('animationend', () => piece.remove());
    layer.appendChild(piece);
}

function modeForBoardSize(boardSize) {
    if (boardSize === 8) return 'classic';
    if (boardSize === 10) return 'modern10';
    if (boardSize === 12) return 'modern12';
    return 'classic';
}

function modeLabel(mode, boardSize = null) {
    if (mode === 'classic') return boardSize && boardSize !== 8 ? `classic ${boardSize}×${boardSize}` : 'classic 8×8';
    if (mode === 'modern10') return 'modern 10×10';
    if (mode === 'modern12') return 'modern 12×12';
    return boardSize ? `${mode} ${boardSize}×${boardSize}` : mode;
}

function ratingApiMode() {
    return 'classic';
}

function buildCurrentVisualStatePayload() {
    return {
        version: 1,
        light_square: { id: settings.lightSquareStrategyId },
        dark_square: { id: settings.darkSquareStrategyId },
        pieces: {
            light: { ...settings.lightPieceStrategyByType },
            dark: { ...settings.darkPieceStrategyByType }
        },
        background: { id: settings.backgroundStrategyId },
        falling_pieces_enabled: Boolean(settings.fallingPiecesEnabled)
    };
}

async function ensureMatchAuthentication() {
    if (!window.ChessApi?.hasToken?.()) {
        navigateTo('page-account');
        showAccountMessage('Log in before searching for a match.');
        return false;
    }

    if (!accountProfile.signedIn || !accountProfile.username) {
        const refreshed = await refreshAccountFromBackend();
        if (!refreshed) {
            navigateTo('page-account');
            showAccountMessage('Log in before searching for a match.');
            return false;
        }
    }

    return true;
}

function bindClassicSetupControls() {
    document.querySelectorAll('[data-time-control]').forEach(button => {
        button.addEventListener('click', () => {
            selectedClassicTimeMinutes = Number(button.dataset.timeControl);
            renderClassicSetupSelection();
        });
    });

    document.querySelectorAll('[data-classic-ranked]').forEach(button => {
        button.addEventListener('click', () => {
            selectedClassicIsRanked = button.dataset.classicRanked === 'true';
            renderClassicSetupSelection();
        });
    });

    document.getElementById('classic-start-btn')?.addEventListener('click', async () => {
        if (!selectedClassicTimeMinutes) return;
        if (!await ensureMatchAuthentication()) return;
        renderClassicBoard(8, selectedClassicTimeMinutes, true, true, 'classic', selectedClassicIsRanked);
    });
}

function bindModernSetupControls() {
    document.querySelectorAll('[data-modern-board-size]').forEach(button => {
        button.addEventListener('click', () => {
            selectedModernBoardSize = Number(button.dataset.modernBoardSize);
            renderModernSetupSelection();
        });
    });

    document.querySelectorAll('[data-modern-time-control]').forEach(button => {
        button.addEventListener('click', () => {
            selectedModernTimeMinutes = Number(button.dataset.modernTimeControl);
            renderModernSetupSelection();
        });
    });

    document.querySelectorAll('[data-modern-ranked]').forEach(button => {
        button.addEventListener('click', () => {
            selectedModernIsRanked = button.dataset.modernRanked === 'true';
            renderModernSetupSelection();
        });
    });

    document.getElementById('modern-start-btn')?.addEventListener('click', async () => {
        if (!selectedModernBoardSize || !selectedModernTimeMinutes) return;
        if (!await ensureMatchAuthentication()) return;
        const boardSize = selectedModernBoardSize;
        const timeControl = selectedModernTimeMinutes;
        const isRanked = selectedModernIsRanked;
        navigateTo('page-classic');
        renderClassicBoard(boardSize, timeControl, true, true, modeForBoardSize(boardSize), isRanked);
    });
}

function renderClassicSetupSelection() {
    document.querySelectorAll('[data-time-control]').forEach(button => {
        button.classList.toggle('active', Number(button.dataset.timeControl) === selectedClassicTimeMinutes);
    });

    document.querySelectorAll('[data-classic-ranked]').forEach(button => {
        button.classList.toggle('active', (button.dataset.classicRanked === 'true') === selectedClassicIsRanked);
    });

    const startButton = document.getElementById('classic-start-btn');
    if (startButton) {
        startButton.disabled = !selectedClassicTimeMinutes;
    }
}

function renderModernSetupSelection() {
    document.querySelectorAll('[data-modern-board-size]').forEach(button => {
        button.classList.toggle('active', Number(button.dataset.modernBoardSize) === selectedModernBoardSize);
    });

    document.querySelectorAll('[data-modern-time-control]').forEach(button => {
        button.classList.toggle('active', Number(button.dataset.modernTimeControl) === selectedModernTimeMinutes);
    });

    document.querySelectorAll('[data-modern-ranked]').forEach(button => {
        button.classList.toggle('active', (button.dataset.modernRanked === 'true') === selectedModernIsRanked);
    });

    const startButton = document.getElementById('modern-start-btn');
    if (startButton) {
        startButton.disabled = !selectedModernBoardSize || !selectedModernTimeMinutes;
    }
}

function bindRatingControls() {
    document.querySelectorAll('[data-rating-board-size]').forEach(button => {
        button.addEventListener('click', () => {
            const boardSize = Number(button.dataset.ratingBoardSize);
            if (!boardSize || ratingsState.boardSize === boardSize) return;
            ratingsState.boardSize = boardSize;
            loadRatingPage({ force: true });
        });
    });

    document.querySelectorAll('[data-rating-time-control]').forEach(button => {
        button.addEventListener('click', () => {
            const minutes = Number(button.dataset.ratingTimeControl);
            if (!minutes || ratingsState.timeControlMinutes === minutes) return;
            ratingsState.timeControlMinutes = minutes;
            loadRatingPage({ force: true });
        });
    });

    document.getElementById('rating-refresh-btn')?.addEventListener('click', () => {
        loadRatingPage({ force: true });
    });

    renderRatingPage();
}

function resetRatingState() {
    ratingsState = {
        boardSize: 8,
        timeControlMinutes: 10,
        loading: false,
        error: '',
        leaderboard: null
    };
    renderRatingPage();
}

async function loadRatingPage({ force = false } = {}) {
    if (ratingsState.loading) {
        renderRatingPage();
        return;
    }

    if (ratingsState.leaderboard && !force) {
        renderRatingPage();
        return;
    }

    ratingsState.loading = true;
    ratingsState.error = '';
    renderRatingPage();

    try {
        ratingsState.leaderboard = await ChessApi.leaderboard({
            mode: ratingApiMode(ratingsState.boardSize),
            boardSize: ratingsState.boardSize,
            timeLimitMinutes: ratingsState.timeControlMinutes,
            limit: 50
        });
    } catch (error) {
        ratingsState.leaderboard = null;
        ratingsState.error = window.ChessApi?.getErrorMessage?.(error) || playerFacingErrorMessage(error, 'Could not load the leaderboard.');
    } finally {
        ratingsState.loading = false;
        renderRatingPage();
    }
}

function renderRatingPage() {
    document.querySelectorAll('[data-rating-board-size]').forEach(button => {
        button.classList.toggle('active', Number(button.dataset.ratingBoardSize) === ratingsState.boardSize);
    });
    document.querySelectorAll('[data-rating-time-control]').forEach(button => {
        button.classList.toggle('active', Number(button.dataset.ratingTimeControl) === ratingsState.timeControlMinutes);
    });

    const scopeLabel = document.getElementById('rating-scope-label');
    if (scopeLabel) {
        scopeLabel.textContent = `${modeLabel(ratingApiMode(ratingsState.boardSize), ratingsState.boardSize)} · ${ratingsState.timeControlMinutes} min`;
    }

    const list = document.getElementById('rating-leaderboard');
    if (!list) return;

    list.innerHTML = '';
    if (ratingsState.loading) {
        renderRatingStateMessage(list, 'Loading leaderboard...');
        return;
    }

    if (ratingsState.error) {
        renderRatingStateMessage(list, ratingsState.error);
        return;
    }

    const players = Array.isArray(ratingsState.leaderboard?.players) ? ratingsState.leaderboard.players : [];
    if (players.length === 0) {
        renderRatingStateMessage(list, 'No ranked games in this category yet.');
        return;
    }

    const header = document.createElement('div');
    header.className = 'rating-row rating-row-header';
    header.append(
        createTextSpan('Rank'),
        createTextSpan('Player'),
        createTextSpan('Rating'),
        createTextSpan('Games')
    );
    list.appendChild(header);

    players.forEach(player => {
        const row = document.createElement('div');
        row.className = 'rating-row';

        const username = player.username || 'Player';
        const isMe = accountProfile.id && player.user_id === accountProfile.id;
        row.append(
            createTextSpan(`#${player.rank || '-'}`),
            createTextStrong(isMe ? `${username} (you)` : username),
            createTextStrong(String(player.rating ?? '-')),
            createTextSpan(String(player.games_played ?? 0))
        );
        list.appendChild(row);
    });
}

function renderRatingStateMessage(list, message) {
    const empty = document.createElement('div');
    empty.className = 'history-empty';
    empty.textContent = message;
    list.appendChild(empty);
}

function createTextSpan(text) {
    const span = document.createElement('span');
    span.textContent = text;
    return span;
}

function createTextStrong(text) {
    const strong = document.createElement('strong');
    strong.textContent = text;
    return strong;
}

function resetModernSetup() {
    selectedModernBoardSize = null;
    selectedModernTimeMinutes = null;
    selectedModernIsRanked = false;
    renderModernSetupSelection();
}

function resetClassicEntry() {
    if (activeRemoteGame && currentGameState?.status === 'active' && window.ChessSocket?.isOpen?.()) {
        try {
            ChessSocket.leaveGame?.();
        } catch (error) {
            console.warn('Unable to notify backend about leaving game', error);
        }
    }

    destroyBoard();
    cancelMatchmaking();
    stopGameTimer();
    hideMatchNotFoundOverlay();
    hideGameFinishedOverlay();
    currentVisualBoardSize = null;
    currentTimeControlMinutes = null;
    currentGameMode = 'classic';
    currentCustomPosition = null;
    selectedCustomSquare = null;
    selectedClassicBoardSize = 8;
    selectedClassicTimeMinutes = null;
    selectedClassicIsRanked = false;
    activeMatchRequest = null;
    queuedForMatch = false;
    activeRemoteGame = false;
    currentGameState = null;
    currentPlayerColor = null;
    currentGameId = null;
    currentIsRanked = false;
    currentValidMoves = {};
    pendingClassicMove = null;
    pendingPromotionMove = null;
    classicSnapbackInProgress = false;
    queuedClassicPositionUpdate = null;
    clearClassicMoveHighlights();
    hidePromotionPicker();
    resetDrawOfferState();
    resetNetworkWarning();
    capturedByMe = [];
    capturedByOpponent = [];
    emojiMessages = [];
    renderClassicSetupSelection();
    renderCapturedPieces();
    renderEmojiMessages();
    renderAllTimers(0, 0);
    setMatchmakingStatus('');
    document.getElementById('classic-setup')?.classList.remove('hidden');
    document.getElementById('classic-board-shell')?.classList.add('hidden');

    const host = document.getElementById('myBoard');
    if (host) {
        host.innerHTML = '';
        host.className = 'board-host';
        host.removeAttribute('style');
    }
}

function renderClassicBoard(size, timeControlMinutes, resetPosition = false, restartSession = true, mode = currentGameMode, isRanked = currentIsRanked) {
    const preservedClassicPosition = !resetPosition && size === 8 && board ? board.position() : null;
    destroyBoard();
    currentVisualBoardSize = size;
    currentTimeControlMinutes = timeControlMinutes;
    currentGameMode = mode;
    currentIsRanked = Boolean(isRanked);

    if (restartSession) {
        stopGameTimer();
        ensureEmojiChat();
        resetEmojiChatSession();
        capturedByMe = [];
        capturedByOpponent = [];
        renderCapturedPieces();
        renderPieceLegend();
        startMatchmaking(size, timeControlMinutes, mode, currentIsRanked);
        startGameTimer(timeControlMinutes);
    }

    document.getElementById('classic-setup')?.classList.add('hidden');
    document.getElementById('classic-board-shell')?.classList.remove('hidden');
    const label = document.getElementById('classic-board-size-label');
    if (label) {
        label.textContent = `${size}×${size} · ${timeControlMinutes} min · ${currentIsRanked ? 'ranked' : 'casual'}`;
    }

    const host = document.getElementById('myBoard');
    if (!host) return;

    host.innerHTML = '';
    host.className = 'board-host';
    host.style.width = '';
    applySelectedBoardSquares();

    if (size === 8) {
        currentCustomPosition = null;
        const initialClassicPosition = isMatchmakingSearchPreview()
            ? {}
            : preservedClassicPosition || 'start';
        host.style.width = 'var(--classic-board-size)';
        board = Chessboard('myBoard', {
            draggable: true,
            dropOffBoard: 'snapback',
            orientation: currentBoardOrientation(),
            position: initialClassicPosition,
            pieceTheme: pieceTheme,
            onDragStart: handleClassicDragStart,
            onDrop: handleClassicDrop,
            onSnapbackEnd: handleClassicSnapbackEnd
        });
        paintRenderedClassicSquares();
        requestAnimationFrame(paintRenderedClassicSquares);
        return;
    }

    if (resetPosition || !currentCustomPosition) {
        currentCustomPosition = isMatchmakingSearchPreview() ? {} : buildVisualPosition(size);
        selectedCustomSquare = null;
    }
    renderCustomBoard(host, size, currentCustomPosition);
}

function destroyBoard() {
    cancelCustomDrag();
    if (board) {
        board.destroy();
        board = null;
    }
}

function startMatchmaking(boardSize, timeControlMinutes, mode = currentGameMode, isRanked = currentIsRanked) {
    setMatchmakingStatus('Searching...');
    hideGameFinishedOverlay();
    activeMatchRequest = { mode, boardSize, timeControlMinutes, isRanked: Boolean(isRanked) };
    currentIsRanked = Boolean(isRanked);
    queuedForMatch = false;
    activeRemoteGame = false;
    currentGameState = null;
    currentPlayerColor = null;
    currentGameId = null;
    pendingPromotionMove = null;
    currentValidMoves = {};
    pendingClassicMove = null;
    classicSnapbackInProgress = false;
    queuedClassicPositionUpdate = null;
    clearClassicMoveHighlights();
    hidePromotionPicker();
    resetDrawOfferState();
    resetNetworkWarning();

    matchmakingClient.findMatch({ mode, boardSize, timeControlMinutes, isRanked })
        .then(result => {
            if (!isCurrentMatchRequest(mode, boardSize, timeControlMinutes)) return;
            setMatchmakingStatus(result.message);
        })
        .catch(error => {
            if (!isCurrentMatchRequest(mode, boardSize, timeControlMinutes)) return;
            activeMatchRequest = null;
            queuedForMatch = false;
            pendingClassicMove = null;
            pendingPromotionMove = null;
            classicSnapbackInProgress = false;
            queuedClassicPositionUpdate = null;
            clearClassicMoveHighlights();
            hidePromotionPicker();
            resetDrawOfferState();
            resetNetworkWarning();
            stopGameTimer();
            setMatchmakingStatus(playerFacingErrorMessage(error, 'Could not start matchmaking. Try again.'));
        });
}

function cancelMatchmaking() {
    if (queuedForMatch) {
        matchmakingClient.cancel();
    }
    queuedForMatch = false;
    activeMatchRequest = null;
    pendingClassicMove = null;
    pendingPromotionMove = null;
    classicSnapbackInProgress = false;
    queuedClassicPositionUpdate = null;
    clearClassicMoveHighlights();
    hidePromotionPicker();
    hideGameFinishedOverlay();
    resetDrawOfferState();
    resetNetworkWarning();
}

function isCurrentMatchRequest(mode, boardSize, timeControlMinutes, isRanked = activeMatchRequest?.isRanked) {
    return activeMatchRequest?.mode === mode
        && activeMatchRequest?.boardSize === boardSize
        && activeMatchRequest?.timeControlMinutes === timeControlMinutes
        && activeMatchRequest?.isRanked === Boolean(isRanked);
}

function bindSocketEvents() {
    if (!window.ChessSocket) return;

    ChessSocket.on('QUEUE_JOINED', handleQueueJoined);
    ChessSocket.on('MATCH_FOUND', handleMatchFound);
    ChessSocket.on('GAME_STATE', handleGameState);
    ChessSocket.on('ERROR', handleSocketProtocolError);
    ChessSocket.on('MOVE_REJECTED', handleMoveRejected);
    ChessSocket.on('CLOSE', handleSocketClose);
    ChessSocket.on('DRAW_OFFER', handleDrawOfferMessage);
    ChessSocket.on('DRAW_ACCEPTED', handleDrawAcceptedMessage);
    ChessSocket.on('DRAW_DECLINE', handleDrawDeclinedMessage);
    ChessSocket.on('DRAW_EXPIRED', handleDrawExpiredMessage);
    ChessSocket.on('CHAT_STICKER', handleChatStickerMessage);
    ChessSocket.on('PLAYER_NETWORK_WAITING', handlePlayerNetworkWaiting);
    ChessSocket.on('PLAYER_NETWORK_RESTORED', handlePlayerNetworkRestored);
}

function handleQueueJoined(payload) {
    queuedForMatch = true;
    const boardSize = payload?.board_size || activeMatchRequest?.boardSize || currentVisualBoardSize || 8;
    const mode = payload?.mode || activeMatchRequest?.mode || currentGameMode;
    const minutes = payload?.time_limit_minutes || activeMatchRequest?.timeControlMinutes || currentTimeControlMinutes;
    currentIsRanked = Boolean(payload?.is_ranked ?? activeMatchRequest?.isRanked ?? currentIsRanked);
    setMatchmakingStatus(`Searching for ${modeLabel(mode, boardSize)} · ${minutes} min · ${currentIsRanked ? 'ranked' : 'casual'}.`);
}

function handleMatchFound(payload) {
    queuedForMatch = false;
    activeRemoteGame = true;
    currentGameState = null;
    currentGameId = payload?.game_id || null;
    currentPlayerColor = payload?.player_color || null;
    currentValidMoves = {};
    pendingClassicMove = null;
    pendingPromotionMove = null;
    classicSnapbackInProgress = false;
    queuedClassicPositionUpdate = null;
    clearClassicMoveHighlights();
    hidePromotionPicker();
    hideGameFinishedOverlay();
    resetDrawOfferState();
    resetNetworkWarning();

    const boardSize = payload?.board_size || activeMatchRequest?.boardSize || currentVisualBoardSize || 8;
    const minutes = payload?.time_limit_minutes || activeMatchRequest?.timeControlMinutes || currentTimeControlMinutes || 10;
    const mode = payload?.mode || activeMatchRequest?.mode || currentGameMode;
    currentIsRanked = Boolean(payload?.is_ranked ?? activeMatchRequest?.isRanked ?? currentIsRanked);

    if (currentVisualBoardSize !== boardSize || currentGameMode !== mode) {
        renderClassicBoard(boardSize, minutes, true, false, mode, currentIsRanked);
    }

    syncRenderedBoardOrientation(boardSize);

    const colorLabel = currentPlayerColor || 'unknown color';
    const opponent = payload?.opponent?.username || 'opponent';
    setMatchmakingStatus(`Match found vs ${opponent}. You play ${colorLabel}. ${currentIsRanked ? 'Ranked' : 'Casual'} game.`);
}

function handleGameState(gameState) {
    if (!gameState) return;

    const boardSize = boardSizeFromGameState(gameState);
    const animateConfirmedClassicMove = boardSize === 8 && isConfirmedPendingClassicMove(gameState);

    activeRemoteGame = true;
    queuedForMatch = false;
    currentGameState = gameState;
    currentGameId = gameState.game_id || currentGameId;
    currentPlayerColor = gameState.player_color || currentPlayerColor;
    currentValidMoves = normalizeValidMoves(gameState.valid_moves);
    clearClassicMoveHighlights();
    clearCustomMoveHighlights();

    const mode = activeMatchRequest?.mode || currentGameMode || modeForBoardSize(boardSize);
    const minutes = currentTimeControlMinutes || Math.max(
        Math.ceil((gameState.white_time_left_ms || 0) / 60000),
        Math.ceil((gameState.black_time_left_ms || 0) / 60000),
        1
    );

    ensureBoardForGameState(boardSize, minutes, mode);
    syncRenderedBoardOrientation(boardSize);
    applyPositionFromGameState(gameState, boardSize, animateConfirmedClassicMove);
    pendingClassicMove = null;
    pendingPromotionMove = null;
    hidePromotionPicker();
    applyCapturedPiecesFromGameState(gameState);
    startServerGameTimer(gameState);

    const status = gameState.status || 'active';
    if (status !== 'active') {
        resetDrawOfferState();
        resetNetworkWarning();
        stopGameTimer();
        refreshPostGameData();
        scheduleGameFinishedOverlay(gameState);
    } else {
        hideGameFinishedOverlay();
        renderDrawOfferControls();
        renderNetworkWarning();
    }

    const turnLabel = status === 'active'
        ? (gameState.turn === currentPlayerColor ? 'Your turn' : 'Opponent turn')
        : 'Game finished';
    setMatchmakingStatus(`${status} · ${turnLabel}`);
}

function handleSocketProtocolError(payload) {
    const message = playerFacingSocketMessage(payload, 'Something went wrong in the match room. Try again.');
    setMatchmakingStatus(message);

    if (payload?.code === 'UNKNOWN_MODE') {
        queuedForMatch = false;
        activeMatchRequest = null;
        currentGameState = null;
        currentValidMoves = {};
        pendingClassicMove = null;
        pendingPromotionMove = null;
        classicSnapbackInProgress = false;
        queuedClassicPositionUpdate = null;
        clearClassicMoveHighlights();
        clearCustomMoveHighlights();
        hidePromotionPicker();
        resetDrawOfferState();
        resetNetworkWarning();
        stopGameTimer();
    }
}

function handleMoveRejected(payload) {
    pendingClassicMove = null;
    pendingPromotionMove = null;
    queuedClassicPositionUpdate = null;
    clearClassicMoveHighlights();
    clearCustomMoveHighlights();
    hidePromotionPicker();
    setMatchmakingStatus(playerFacingSocketMessage(payload, 'That move is not legal.'));
}

function handleSocketClose() {
    if (queuedForMatch) {
        queuedForMatch = false;
        activeMatchRequest = null;
        pendingClassicMove = null;
        pendingPromotionMove = null;
        classicSnapbackInProgress = false;
        queuedClassicPositionUpdate = null;
        clearClassicMoveHighlights();
        hidePromotionPicker();
        stopGameTimer();
        setMatchmakingStatus('Search stopped because the game connection closed. Try again.');
    }
}

function bindGameActionControls() {
    document.getElementById('draw-offer-btn')?.addEventListener('click', () => {
        if (!canUseGameAction()) return;
        if (activeDrawOffer) {
            setMatchmakingStatus('Draw offer is already active.');
            return;
        }

        try {
            ChessSocket.offerDraw();
            setMatchmakingStatus('Draw offer sent.');
        } catch (error) {
            setMatchmakingStatus(playerFacingErrorMessage(error, 'Could not offer a draw.'));
        }
    });

    document.getElementById('draw-accept-btn')?.addEventListener('click', () => {
        if (!canRespondToDrawOffer()) return;
        try {
            ChessSocket.acceptDraw();
            setMatchmakingStatus('Draw accepted.');
        } catch (error) {
            setMatchmakingStatus(playerFacingErrorMessage(error, 'Could not accept the draw.'));
        }
    });

    document.getElementById('draw-decline-btn')?.addEventListener('click', () => {
        if (!canRespondToDrawOffer()) return;
        try {
            ChessSocket.declineDraw();
            setMatchmakingStatus('Draw declined.');
        } catch (error) {
            setMatchmakingStatus(playerFacingErrorMessage(error, 'Could not decline the draw.'));
        }
    });

    renderDrawOfferControls();
    renderNetworkWarning();
}

function canUseGameAction() {
    if (!activeRemoteGame || currentGameState?.status !== 'active') {
        setMatchmakingStatus('Game is not active.');
        return false;
    }

    if (!window.ChessSocket?.isOpen?.()) {
        setMatchmakingStatus(GAME_CONNECTION_LOST_MESSAGE);
        return false;
    }

    return true;
}

function canRespondToDrawOffer() {
    if (!canUseGameAction()) return false;
    if (!activeDrawOffer) {
        setMatchmakingStatus('No active draw offer.');
        return false;
    }
    if (isOwnDrawOffer(activeDrawOffer)) {
        setMatchmakingStatus('Opponent must respond to your draw offer.');
        return false;
    }
    return true;
}

function handleDrawOfferMessage(payload = {}) {
    activeDrawOffer = normalizeDrawOffer(payload);
    const message = payload.message || (isOwnDrawOffer(activeDrawOffer) ? 'You offered a draw.' : 'Opponent offered a draw.');
    appendSystemEmojiMessage(message);
    setMatchmakingStatus(message);
    startDrawOfferCountdown();
    renderDrawOfferControls();
}

function handleDrawAcceptedMessage(payload = {}) {
    const message = payload.message || 'Draw offer accepted. Game finished as draw.';
    appendSystemEmojiMessage(message);
    setMatchmakingStatus(message);
    resetDrawOfferState();
}

function handleDrawDeclinedMessage(payload = {}) {
    const message = payload.message || 'Draw offer declined.';
    appendSystemEmojiMessage(message);
    setMatchmakingStatus(message);
    resetDrawOfferState();
}

function handleDrawExpiredMessage(payload = {}) {
    const message = payload.message || 'Draw offer expired.';
    appendSystemEmojiMessage(message);
    setMatchmakingStatus(message);
    resetDrawOfferState();
}

function normalizeDrawOffer(payload = {}) {
    const fallbackExpiresAt = payload.expires_in_ms
        ? new Date(Date.now() + Number(payload.expires_in_ms)).toISOString()
        : '';

    return {
        offerId: payload.offer_id || '',
        offeredBy: payload.offered_by || '',
        offeredByUserId: payload.offered_by_user_id || '',
        expiresAt: payload.expires_at || fallbackExpiresAt,
        expiresInMs: Number(payload.expires_in_ms || 0),
        message: payload.message || ''
    };
}

function resetDrawOfferState() {
    activeDrawOffer = null;
    if (drawOfferIntervalId) {
        window.clearInterval(drawOfferIntervalId);
        drawOfferIntervalId = null;
    }
    renderDrawOfferControls();
}

function startDrawOfferCountdown() {
    if (drawOfferIntervalId) {
        window.clearInterval(drawOfferIntervalId);
    }
    drawOfferIntervalId = window.setInterval(() => {
        renderDrawOfferControls();
        if (activeDrawOffer && drawOfferRemainingSeconds(activeDrawOffer) <= 0) {
            resetDrawOfferState();
        }
    }, 500);
}

function renderDrawOfferControls() {
    const button = document.getElementById('draw-offer-btn');
    const panel = document.getElementById('draw-offer-panel');
    const message = document.getElementById('draw-offer-message');
    const countdown = document.getElementById('draw-offer-countdown');
    const responseActions = document.getElementById('draw-response-actions');
    const isGameActive = activeRemoteGame && currentGameState?.status === 'active';
    const ownOffer = activeDrawOffer ? isOwnDrawOffer(activeDrawOffer) : false;

    if (button) {
        button.disabled = !isGameActive || !window.ChessSocket?.isOpen?.() || Boolean(activeDrawOffer);
        button.textContent = activeDrawOffer && ownOffer ? 'Draw Offered' : 'Offer Draw';
    }

    if (!panel) return;

    panel.classList.toggle('hidden', !activeDrawOffer);
    if (!activeDrawOffer) return;

    const seconds = drawOfferRemainingSeconds(activeDrawOffer);
    if (message) {
        message.textContent = activeDrawOffer.message || (ownOffer ? 'You offered a draw.' : 'Opponent offered a draw.');
    }
    if (countdown) {
        countdown.textContent = seconds > 0 ? `Expires in ${seconds}s` : 'Expired';
    }
    responseActions?.classList.toggle('hidden', ownOffer);
}

function drawOfferRemainingSeconds(offer) {
    if (!offer) return 0;
    const expiresAtMs = Date.parse(offer.expiresAt || '');
    if (Number.isFinite(expiresAtMs)) {
        return Math.max(0, Math.ceil((expiresAtMs - Date.now()) / 1000));
    }
    return Math.max(0, Math.ceil(Number(offer.expiresInMs || 0) / 1000));
}

function isOwnDrawOffer(offer) {
    return Boolean(
        offer
        && ((offer.offeredByUserId && accountProfile.id && offer.offeredByUserId === accountProfile.id)
            || (offer.offeredBy && currentPlayerColor && offer.offeredBy === currentPlayerColor))
    );
}

function handlePlayerNetworkWaiting(payload = {}) {
    networkWarning = normalizeNetworkWarning(payload);
    const message = playerFacingGameMessage(payload.message, 'Waiting for the other player to reconnect.');
    appendSystemEmojiMessage(message);
    setMatchmakingStatus(message);
    startNetworkWarningCountdown();
    renderNetworkWarning();
}

function handlePlayerNetworkRestored(payload = {}) {
    const message = playerFacingGameMessage(payload.message, 'The other player is back online.');
    appendSystemEmojiMessage(message);
    setMatchmakingStatus(message);
    resetNetworkWarning();
}

function normalizeNetworkWarning(payload = {}) {
    const fallbackExpiresAt = payload.remaining_ms
        ? new Date(Date.now() + Number(payload.remaining_ms)).toISOString()
        : '';

    return {
        userId: payload.user_id || '',
        username: payload.username || '',
        color: payload.color || '',
        expiresAt: payload.expires_at || fallbackExpiresAt,
        remainingMs: Number(payload.remaining_ms || 0),
        message: payload.message || ''
    };
}

function resetNetworkWarning() {
    networkWarning = null;
    if (networkWarningIntervalId) {
        window.clearInterval(networkWarningIntervalId);
        networkWarningIntervalId = null;
    }
    renderNetworkWarning();
}

function startNetworkWarningCountdown() {
    if (networkWarningIntervalId) {
        window.clearInterval(networkWarningIntervalId);
    }
    networkWarningIntervalId = window.setInterval(() => {
        renderNetworkWarning();
        if (networkWarning && networkWarningRemainingSeconds(networkWarning) <= 0) {
            resetNetworkWarning();
        }
    }, 500);
}

function renderNetworkWarning() {
    const panel = document.getElementById('network-warning');
    if (!panel) return;

    panel.classList.toggle('hidden', !networkWarning);
    if (!networkWarning) {
        panel.textContent = '';
        return;
    }

    const seconds = networkWarningRemainingSeconds(networkWarning);
    const baseMessage = networkWarning.message || 'Waiting for player network to recover.';
    panel.textContent = seconds > 0 ? `${baseMessage} ${seconds}s left.` : baseMessage;
}

function networkWarningRemainingSeconds(warning) {
    const expiresAtMs = Date.parse(warning?.expiresAt || '');
    if (Number.isFinite(expiresAtMs)) {
        return Math.max(0, Math.ceil((expiresAtMs - Date.now()) / 1000));
    }
    return Math.max(0, Math.ceil(Number(warning?.remainingMs || 0) / 1000));
}

function boardSizeFromGameState(gameState) {
    return gameState?.board_size
        || gameState?.board?.width
        || gameState?.board?.height
        || currentVisualBoardSize
        || 8;
}

function normalizeValidMoves(validMoves) {
    return Object.entries(validMoves || {}).reduce((moves, [from, targets]) => {
        if (Array.isArray(targets)) {
            moves[from] = targets.filter(Boolean);
        }
        return moves;
    }, {});
}

function ensureBoardForGameState(boardSize, timeControlMinutes, mode) {
    const boardShell = document.getElementById('classic-board-shell');
    const isBoardVisible = boardShell && !boardShell.classList.contains('hidden');
    const needsRender = currentVisualBoardSize !== boardSize
        || currentTimeControlMinutes !== timeControlMinutes
        || currentGameMode !== mode
        || !isBoardVisible;

    if (needsRender) {
        renderClassicBoard(boardSize, timeControlMinutes, true, false, mode);
    }
}

function currentBoardOrientation() {
    return currentPlayerColor === 'black' ? 'black' : 'white';
}

function isMatchmakingSearchPreview() {
    return Boolean(activeMatchRequest && !activeRemoteGame && !currentGameState);
}

function syncRenderedBoardOrientation(boardSize = currentVisualBoardSize) {
    const orientation = currentBoardOrientation();

    if (boardSize === 8 && board) {
        if (board.orientation() !== orientation) {
            board.orientation(orientation);
            paintRenderedClassicSquares();
            requestAnimationFrame(paintRenderedClassicSquares);
        }
        return;
    }

    if (boardSize && boardSize !== 8 && currentCustomPosition) {
        refreshCurrentBoard(false);
    }
}

function applyPositionFromGameState(gameState, boardSize, animateClassicMove = false) {
    const pieces = gameState?.board?.pieces || [];
    if (boardSize === 8) {
        const position = backendPiecesToChessboardPosition(pieces);
        if (!board) {
            renderClassicBoard(boardSize, currentTimeControlMinutes || 10, true, false, currentGameMode);
        }
        if (animateClassicMove && classicSnapbackInProgress) {
            queuedClassicPositionUpdate = { position, animate: true };
            return;
        }
        queuedClassicPositionUpdate = null;
        applyClassicBoardPosition(position, animateClassicMove);
        return;
    }

    currentCustomPosition = backendPiecesToCustomPosition(pieces, boardSize);
    selectedCustomSquare = null;
    refreshCurrentBoard(false);
}

function applyClassicBoardPosition(position, animate = false) {
    board?.position(position, animate);
    paintRenderedClassicSquares();
    requestAnimationFrame(paintRenderedClassicSquares);
}

function flushQueuedClassicPositionUpdate() {
    const update = queuedClassicPositionUpdate;
    if (!update) return;

    queuedClassicPositionUpdate = null;
    applyClassicBoardPosition(update.position, update.animate);
}

function isConfirmedPendingClassicMove(gameState) {
    const lastMove = gameState?.last_move;
    return Boolean(
        pendingClassicMove
        && lastMove?.from === pendingClassicMove.from
        && lastMove?.to === pendingClassicMove.to
    );
}

function backendPiecesToChessboardPosition(pieces) {
    return pieces.reduce((position, piece) => {
        const code = backendPieceToFrontendCode(piece);
        if (code && piece.square) {
            position[piece.square] = code;
        }
        return position;
    }, {});
}

function backendPiecesToCustomPosition(pieces, boardSize) {
    return pieces.reduce((position, piece) => {
        const code = backendPieceToFrontendCode(piece);
        const square = backendSquareToCustomKey(piece.square, boardSize);
        if (code && square) {
            position[square] = code;
        }
        return position;
    }, {});
}

function backendPieceToFrontendCode(piece) {
    if (!piece?.type || !piece?.color) return '';

    const color = piece.color === 'white' ? 'w' : 'b';
    const typeMap = {
        pawn: 'P',
        rook: 'R',
        knight: 'N',
        horse: 'N',
        bishop: 'B',
        queen: 'Q',
        king: 'K'
    };
    const type = typeMap[piece.type];
    return type ? `${color}${type}` : '';
}

function backendSquareToCustomKey(square, boardSize) {
    if (!square || square.length < 2) return '';

    const file = square.charCodeAt(0) - 'a'.charCodeAt(0);
    const rank = Number(square.slice(1));
    if (!Number.isInteger(rank) || file < 0 || file >= boardSize || rank < 1 || rank > boardSize) return '';

    return squareKey(boardSize - rank, file);
}

function applyCapturedPiecesFromGameState(gameState) {
    const capturedWhite = gameState?.captured_white || [];
    const capturedBlack = gameState?.captured_black || [];

    if (currentPlayerColor === 'black') {
        capturedByMe = capturedWhite.map(backendPieceToFrontendCode).filter(Boolean);
        capturedByOpponent = capturedBlack.map(backendPieceToFrontendCode).filter(Boolean);
    } else {
        capturedByMe = capturedBlack.map(backendPieceToFrontendCode).filter(Boolean);
        capturedByOpponent = capturedWhite.map(backendPieceToFrontendCode).filter(Boolean);
    }

    renderCapturedPieces();
}

function setMatchmakingStatus(message) {
    const status = document.getElementById('matchmaking-status');
    if (status) {
        status.textContent = message;
    }
}

function startGameTimer(timeControlMinutes) {
    const selectedSeconds = timeControlMinutes * 60;
    const searchSeconds = 60;
    timerState = {
        mode: 'search',
        initialSeconds: searchSeconds,
        remaining: {
            opponent: selectedSeconds,
            me: searchSeconds
        },
        active: 'me',
        lastTickAt: Date.now()
    };
    renderAllTimers(timerState.remaining.opponent, timerState.remaining.me);
    timerIntervalId = window.setInterval(tickGameTimer, GAME_TIMER_TICK_MS);
}

function startServerGameTimer(gameState) {
    if (!gameState) return;

    if (timerIntervalId) {
        window.clearInterval(timerIntervalId);
        timerIntervalId = null;
    }
    if (matchNotFoundTimeoutId) {
        window.clearTimeout(matchNotFoundTimeoutId);
        matchNotFoundTimeoutId = null;
    }

    const whiteSeconds = Math.ceil((gameState.white_time_left_ms || 0) / 1000);
    const blackSeconds = Math.ceil((gameState.black_time_left_ms || 0) / 1000);
    const playerColor = gameState.player_color || currentPlayerColor || 'white';
    const meSeconds = playerColor === 'white' ? whiteSeconds : blackSeconds;
    const opponentSeconds = playerColor === 'white' ? blackSeconds : whiteSeconds;
    const selectedSeconds = Math.max((currentTimeControlMinutes || 0) * 60, meSeconds, opponentSeconds, 1);

    timerState = {
        mode: 'game',
        initialSeconds: selectedSeconds,
        remaining: {
            opponent: opponentSeconds,
            me: meSeconds
        },
        active: gameState.turn === playerColor ? 'me' : 'opponent',
        lastTickAt: Date.now()
    };

    renderAllTimers(timerState.remaining.opponent, timerState.remaining.me);

    if (gameState.status === 'active') {
        timerIntervalId = window.setInterval(tickGameTimer, GAME_TIMER_TICK_MS);
    }
}

function stopGameTimer() {
    if (timerIntervalId) {
        window.clearInterval(timerIntervalId);
        timerIntervalId = null;
    }
    if (matchNotFoundTimeoutId) {
        window.clearTimeout(matchNotFoundTimeoutId);
        matchNotFoundTimeoutId = null;
    }
    timerState = null;
}

function tickGameTimer() {
    if (!timerState) return;

    const now = Date.now();
    const elapsedSeconds = Math.floor((now - timerState.lastTickAt) / 1000);
    if (elapsedSeconds < 1) return;

    timerState.lastTickAt += elapsedSeconds * 1000;
    const activeTimer = timerState.active || 'me';
    timerState.remaining[activeTimer] = Math.max(0, timerState.remaining[activeTimer] - elapsedSeconds);
    renderAllTimers(timerState.remaining.opponent, timerState.remaining.me);

    if (timerState.mode === 'search' && timerState.remaining.me === 0) {
        window.clearInterval(timerIntervalId);
        timerIntervalId = null;
        showMatchNotFoundOverlay();
    }
}

function handleLocalMoveComplete() {
    // Moves stay local while the match is being searched.
}

function renderAllTimers(opponentSeconds, meSeconds) {
    renderTimer('opponent', opponentSeconds);
    renderTimer('me', meSeconds);
}

function renderTimer(kind, seconds) {
    const timer = document.getElementById(`${kind}-timer`);
    const digits = document.getElementById(`${kind}-timer-digits`);
    if (!timer || !digits) return;

    const initial = timerState?.initialSeconds || Math.max(seconds, 1);
    const isGood = seconds / initial >= 0.1;
    timer.classList.toggle('timer-good', isGood);
    timer.classList.toggle('timer-low', !isGood);
    timer.classList.toggle('active', timerState?.active === kind);

    digits.innerHTML = '';
    formatTimer(seconds).split('').forEach(char => {
        if (/\d/.test(char)) {
            const img = document.createElement('img');
            img.className = 'timer-digit';
            img.src = `${TIMER_DIGIT_ROOT}/${char}.png`;
            img.alt = char;
            digits.appendChild(img);
            return;
        }

        const spacer = document.createElement('span');
        spacer.className = 'timer-colon-spacer';
        spacer.setAttribute('aria-hidden', 'true');
        digits.appendChild(spacer);
    });
}

function showMatchNotFoundOverlay() {
    document.getElementById('match-not-found-overlay')?.classList.remove('hidden');
    setMatchmakingStatus('');
    matchNotFoundTimeoutId = window.setTimeout(() => {
        matchNotFoundTimeoutId = null;
        navigateTo('page-menu');
    }, 1800);
}

function hideMatchNotFoundOverlay() {
    document.getElementById('match-not-found-overlay')?.classList.add('hidden');
    if (matchNotFoundTimeoutId) {
        window.clearTimeout(matchNotFoundTimeoutId);
        matchNotFoundTimeoutId = null;
    }
}

function scheduleGameFinishedOverlay(gameState) {
    if (!gameState || gameState.status === 'active') return;

    clearGameFinishedOverlayTimers();
    const message = gameFinishedMessage(gameState);

    gameFinishedOverlayDelayId = window.setTimeout(() => {
        gameFinishedOverlayDelayId = null;
        showGameFinishedOverlay(message);

        gameFinishedRedirectTimeoutId = window.setTimeout(() => {
            gameFinishedRedirectTimeoutId = null;
            navigateTo('page-menu');
        }, 5000);
    }, 500);
}

function showGameFinishedOverlay(message) {
    const overlay = document.getElementById('game-finished-overlay');
    if (!overlay) return;

    overlay.textContent = message;
    overlay.classList.remove('hidden');
    setMatchmakingStatus('');
}

function hideGameFinishedOverlay() {
    clearGameFinishedOverlayTimers();
    const overlay = document.getElementById('game-finished-overlay');
    if (overlay) {
        overlay.classList.add('hidden');
    }
}

function clearGameFinishedOverlayTimers() {
    if (gameFinishedOverlayDelayId) {
        window.clearTimeout(gameFinishedOverlayDelayId);
        gameFinishedOverlayDelayId = null;
    }
    if (gameFinishedRedirectTimeoutId) {
        window.clearTimeout(gameFinishedRedirectTimeoutId);
        gameFinishedRedirectTimeoutId = null;
    }
}

function gameFinishedMessage(gameState) {
    const status = String(gameState?.status || '').toLowerCase();
    if (status.includes('draw')) {
        return 'Draw';
    }

    const playerColor = gameState?.player_color || currentPlayerColor;
    const winnerColor = status.includes('white_won')
        ? 'white'
        : status.includes('black_won')
            ? 'black'
            : '';

    if (!winnerColor || !playerColor) {
        return 'Game finished';
    }

    if (winnerColor === playerColor) {
        return 'You won';
    }

    return 'You lost';
}

function refreshPostGameData() {
    refreshAccountFromBackend();
    historyLoaded = false;
    if (document.getElementById('page-rating')?.classList.contains('active')) {
        loadRatingPage({ force: true });
    } else {
        ratingsState.leaderboard = null;
    }
}

function formatTimer(totalSeconds) {
    const safeSeconds = Math.max(0, totalSeconds);
    const minutes = String(Math.floor(safeSeconds / 60)).padStart(2, '0');
    const seconds = String(safeSeconds % 60).padStart(2, '0');
    return `${minutes}:${seconds}`;
}

function ensureEmojiChat() {
    if (emojiChatRendered) return;

    const picker = document.getElementById('emoji-chat-picker');
    if (!picker) return;

    picker.innerHTML = '';
    emojiChatItems.forEach(item => {
        const button = document.createElement('button');
        button.type = 'button';
        button.className = 'emoji-chat-btn';
        button.title = item.name;
        button.setAttribute('aria-label', item.name);
        const img = document.createElement('img');
        img.src = item.src;
        img.alt = '';
        button.appendChild(img);
        button.addEventListener('click', () => sendEmojiMessage(item));
        picker.appendChild(button);
    });

    emojiChatRendered = true;
}

function resetEmojiChatSession() {
    emojiMessages = [];
    renderEmojiMessages();
}

function sendEmojiMessage(item) {
    if (activeRemoteGame && currentGameState?.status === 'active' && window.ChessSocket?.isOpen?.()) {
        try {
            ChessSocket.chatSticker(item.id);
        } catch (error) {
            setMatchmakingStatus(playerFacingErrorMessage(error, 'Could not send the sticker.'));
        }
        return;
    }

    emojiMessages.push({
        id: createUserId('chat-me'),
        sender: 'me',
        name: accountProfile.signedIn && accountProfile.username ? accountProfile.username : 'Me',
        label: item.name,
        src: item.src
    });
    renderEmojiMessages();
}

function handleChatStickerMessage(payload = {}) {
    const sticker = emojiChatItems.find(item => item.id === payload.sticker_id);
    const isMine = Boolean(
        (payload.sender_user_id && accountProfile.id && payload.sender_user_id === accountProfile.id)
        || (payload.sender_color && currentPlayerColor && payload.sender_color === currentPlayerColor)
    );

    emojiMessages.push({
        id: payload.message_id || createUserId('chat-sticker'),
        sender: isMine ? 'me' : 'opponent',
        name: payload.sender_username || (isMine ? accountProfile.username || 'Me' : 'Opponent'),
        label: payload.label || sticker?.name || payload.sticker_id || 'Sticker',
        src: payload.src || sticker?.src || ''
    });
    renderEmojiMessages();
}

function bindHistoryControls() {
    document.querySelectorAll('[data-history-sort]').forEach(input => {
        input.addEventListener('change', event => {
            if (!event.currentTarget.checked) {
                event.currentTarget.checked = true;
                return;
            }
            historySortDirection = event.currentTarget.dataset.historySort;
            document.querySelectorAll('[data-history-sort]').forEach(other => {
                other.checked = other === event.currentTarget;
            });
            renderHistoryList();
        });
    });

    document.querySelectorAll('[data-history-filter]').forEach(input => {
        input.addEventListener('change', event => {
            const result = event.currentTarget.dataset.historyFilter;
            if (event.currentTarget.checked) {
                historyFilters.add(result);
            } else {
                historyFilters.delete(result);
            }
            renderHistoryList();
        });
    });

    document.querySelectorAll('[data-history-replay]').forEach(button => {
        button.addEventListener('click', () => moveHistoryReplay(button.dataset.historyReplay));
    });
}

async function loadHistoryList({ force = false } = {}) {
    const list = document.getElementById('history-list');
    if (!list) return;

    if (!window.ChessApi?.hasToken?.()) {
        resetHistoryState();
        renderHistoryList();
        return;
    }

    if (historyLoading) {
        renderHistoryList();
        return;
    }

    if (historyLoaded && !force) {
        renderHistoryList();
        return;
    }

    historyLoading = true;
    historyLoadError = '';
    renderHistoryList();

    try {
        const payload = await ChessApi.listGames();
        historyRecords = normalizeHistoryGames(payload?.games);
        historyLoaded = true;
    } catch (error) {
        if (error?.status === 401) {
            ChessApi.clearToken();
            accountProfile = createEmptyAccountProfile();
            renderAccountProfile();
        }
        historyRecords = [];
        historyLoaded = false;
        historyLoadError = window.ChessApi?.getErrorMessage?.(error) || playerFacingErrorMessage(error, 'Could not load game history.');
    } finally {
        historyLoading = false;
        renderHistoryList();
    }
}

function renderHistoryList() {
    const list = document.getElementById('history-list');
    if (!list) return;

    list.innerHTML = '';

    if (!window.ChessApi?.hasToken?.()) {
        renderHistoryStateMessage(list, 'Log in to view your game history.');
        return;
    }

    if (historyLoading) {
        renderHistoryStateMessage(list, 'Loading game history...');
        return;
    }

    if (historyLoadError) {
        renderHistoryStateMessage(list, historyLoadError, {
            actionLabel: 'Retry',
            onAction: () => loadHistoryList({ force: true })
        });
        return;
    }

    const records = historyRecords
        .filter(record => historyFilters.size === 0 || historyFilters.has(record.result))
        .sort((a, b) => {
            const diff = new Date(a.timestamp) - new Date(b.timestamp);
            return historySortDirection === 'asc' ? diff : -diff;
        });

    if (records.length === 0) {
        const message = historyRecords.length === 0 ? 'No games yet.' : 'No games match selected filters.';
        renderHistoryStateMessage(list, message);
        return;
    }

    const fragment = document.createDocumentFragment();

    records.forEach(record => {
        const button = document.createElement('button');
        button.type = 'button';
        button.className = `history-game-card history-result-${record.result}`;
        button.addEventListener('click', () => openHistoryGame(record.id));

        const board = document.createElement('span');
        board.className = 'history-card-board';
        renderHistoryMiniBoard(board, record);

        const meta = document.createElement('span');
        meta.className = 'history-card-meta';

        const title = document.createElement('strong');
        title.textContent = `${resultLabel(record.result)} vs ${record.opponent}`;

        const format = document.createElement('span');
        format.textContent = `${modeLabel(record.mode, record.boardSize)} · ${record.timeControl} · ${record.isRanked ? 'ranked' : 'casual'}`;

        const timestamp = document.createElement('span');
        timestamp.textContent = formatHistoryDate(record.timestamp);

        meta.append(title, format, timestamp);

        button.append(board, meta);
        fragment.appendChild(button);
    });

    list.appendChild(fragment);
}

function renderHistoryStateMessage(list, message, action = null) {
    const empty = document.createElement('div');
    empty.className = 'history-empty';

    const text = document.createElement('span');
    text.textContent = message;
    empty.appendChild(text);

    if (action?.actionLabel && typeof action.onAction === 'function') {
        const button = document.createElement('button');
        button.type = 'button';
        button.className = 'mini-action-btn history-retry-btn';
        button.textContent = action.actionLabel;
        button.addEventListener('click', action.onAction);
        empty.appendChild(button);
    }

    list.appendChild(empty);
}

function resetHistoryState() {
    historyRecords = [];
    historyLoaded = false;
    historyLoading = false;
    historyLoadError = '';
    activeHistoryDetailRequest += 1;
}

function clearLocalHistoryRecords() {
    try {
        LEGACY_HISTORY_STORAGE_KEYS.forEach(key => localStorage.removeItem(key));
    } catch (error) {
        console.warn('Unable to clear local history records', error);
    }
}

async function openHistoryGame(recordId) {
    const summary = historyRecords.find(item => item.id === recordId);
    if (!recordId || !window.ChessApi?.hasToken?.()) return;

    navigateTo('page-history-detail');
    const requestId = activeHistoryDetailRequest + 1;
    activeHistoryDetailRequest = requestId;

    renderHistoryDetailLoading(summary);

    try {
        const payload = await ChessApi.getGame(recordId);
        if (activeHistoryDetailRequest !== requestId) return;

        const detail = normalizeHistoryGame(payload);
        updateHistoryRecord(detail);
        renderHistoryDetail(detail);
    } catch (error) {
        if (activeHistoryDetailRequest !== requestId) return;
        if (error?.status === 401) {
            ChessApi.clearToken();
            resetHistoryState();
            accountProfile = createEmptyAccountProfile();
            renderAccountProfile();
        }
        renderHistoryDetailError(window.ChessApi?.getErrorMessage?.(error) || playerFacingErrorMessage(error, 'Could not open this game.'));
    }
}

function renderHistoryDetailLoading(record = null) {
    const title = document.getElementById('history-detail-title');
    const format = document.getElementById('history-accuracy');
    const result = document.getElementById('history-result');
    const status = document.getElementById('history-opening');

    if (title) title.textContent = record ? `Game vs ${record.opponent}` : 'Loading game';
    if (format) format.textContent = record ? historyFormatLabel(record) : '-';
    if (result) result.textContent = record ? resultLabel(record.result) : '-';
    if (status) status.textContent = 'Loading...';

    renderHistoryAnalysisBoard(record);
    renderHistoryMoveList({ moves: [] }, 'Loading moves...');
}

function renderHistoryDetail(record) {
    const title = document.getElementById('history-detail-title');
    const format = document.getElementById('history-accuracy');
    const result = document.getElementById('history-result');
    const status = document.getElementById('history-opening');

    if (title) title.textContent = `Game vs ${record.opponent}`;
    if (format) format.textContent = historyFormatLabel(record);
    if (result) result.textContent = resultLabel(record.result);
    if (status) status.textContent = historyStatusLabel(record);

    renderHistoryAnalysisBoard(record);
    renderHistoryMoveList(record);
}

function renderHistoryDetailError(message) {
    const title = document.getElementById('history-detail-title');
    const format = document.getElementById('history-accuracy');
    const result = document.getElementById('history-result');
    const status = document.getElementById('history-opening');

    if (title) title.textContent = 'Game unavailable';
    if (format) format.textContent = '-';
    if (result) result.textContent = '-';
    if (status) status.textContent = 'Unavailable';

    const host = document.getElementById('history-analysis-board');
    if (host) {
        destroyHistoryReplayBoard();
        host.innerHTML = '';
        const empty = document.createElement('div');
        empty.className = 'history-empty';
        empty.textContent = message;
        host.appendChild(empty);
    }

    renderHistoryMoveList({ moves: [] }, message);
}

function renderHistoryMiniBoard(host, record) {
    host.innerHTML = '';
    const size = clampHistoryBoardSize(record?.boardSize || 8);
    const orientation = historyReplayOrientation(record);
    const visualState = record?.visualState || {};
    const position = Object.keys(backendPiecesToHistoryPosition(record?.boardState?.board?.pieces || [])).length > 0
        ? backendPiecesToHistoryPosition(record?.boardState?.board?.pieces || [])
        : buildInitialHistoryPosition(size);

    host.dataset.size = `${size}×${size}`;
    host.style.gridTemplateColumns = `repeat(${size}, 1fr)`;
    host.style.gridTemplateRows = `repeat(${size}, 1fr)`;
    const lightSquare = getVisualSquareStrategy(visualState, 'light');
    const darkSquare = getVisualSquareStrategy(visualState, 'dark');
    const fragment = document.createDocumentFragment();

    for (let row = 0; row < size; row += 1) {
        for (let col = 0; col < size; col += 1) {
            const logicalRow = customLogicalIndex(row, size, orientation);
            const logicalCol = customLogicalIndex(col, size, orientation);
            const squareName = historySquareName(logicalRow, logicalCol, size);
            const square = document.createElement('span');
            const strategy = (logicalRow + logicalCol) % 2 === 0 ? lightSquare : darkSquare;
            square.className = 'history-card-square';
            square.style.backgroundImage = `url("${strategy.getSrc()}")`;
            square.style.backgroundColor = strategy.getColor();

            const piece = position[squareName];
            if (piece) {
                const img = document.createElement('img');
                img.src = getVisualPieceSrc(piece, visualState);
                img.alt = '';
                square.appendChild(img);
            }
            fragment.appendChild(square);
        }
    }

    host.appendChild(fragment);
}

function renderHistoryAnalysisBoard(record) {
    const host = document.getElementById('history-analysis-board');
    if (!host) return;

    destroyHistoryReplayBoard();
    host.innerHTML = '';

    if (!record) {
        const empty = document.createElement('div');
        empty.className = 'history-empty';
        empty.textContent = 'No game selected.';
        host.appendChild(empty);
        updateHistoryReplayControls();
        return;
    }

    historyReplayState = createHistoryReplayState(record);
    renderHistoryReplayPosition({ animate: false });
}

function createHistoryReplayState(record) {
    const size = clampHistoryBoardSize(record?.boardSize || record?.boardState?.board_size || record?.boardState?.board?.width || 8);
    const moves = Array.isArray(record?.moves) ? record.moves : [];

    return {
        record,
        size,
        orientation: historyReplayOrientation(record),
        moves,
        positions: buildHistoryReplayPositions(record, size, moves),
        index: 0
    };
}

function historyReplayOrientation(record) {
    return record?.playerColor === 'black' ? 'black' : 'white';
}

function buildHistoryReplayPositions(record, size, moves) {
    const positions = [buildInitialHistoryPosition(size)];

    moves.forEach(move => {
        positions.push(applyHistoryMove(positions[positions.length - 1], move, size));
    });

    const finalPosition = backendPiecesToHistoryPosition(record?.boardState?.board?.pieces || []);
    if (moves.length > 0 && Object.keys(finalPosition).length > 0) {
        positions[positions.length - 1] = finalPosition;
    }

    return positions;
}

function buildInitialHistoryPosition(size) {
    const rank = buildBackRank(size);
    const position = {};

    rank.forEach((piece, col) => {
        const file = fileLabel(col);
        position[`${file}${size}`] = `b${piece}`;
        position[`${file}${size - 1}`] = 'bP';
        position[`${file}2`] = 'wP';
        position[`${file}1`] = `w${piece}`;
    });

    return position;
}

function applyHistoryMove(position, move, size) {
    const next = { ...position };
    const from = move?.from;
    const to = move?.to;
    if (!from || !to) return next;

    const movingPiece = next[from] || backendPieceToFrontendCode(move.piece);
    if (!movingPiece) return next;

    const pieceAfterMove = frontendPromotionPiece(movingPiece, move.promotion) || backendPieceToFrontendCode(move.piece) || movingPiece;
    const fromSquare = parseHistorySquare(from);
    const toSquare = parseHistorySquare(to);
    const targetHadPiece = Boolean(next[to]);

    if (isHistoryEnPassantMove(move, movingPiece, fromSquare, toSquare, targetHadPiece)) {
        delete next[historySquareFromParts(toSquare.fileIndex, fromSquare.rank)];
    }

    delete next[from];
    next[to] = pieceAfterMove;

    applyHistoryCastlingMove(next, movingPiece, fromSquare, toSquare, size);

    return next;
}

function isHistoryEnPassantMove(move, movingPiece, fromSquare, toSquare, targetHadPiece) {
    return Boolean(
        move?.captured
        && movingPiece[1] === 'P'
        && fromSquare
        && toSquare
        && fromSquare.fileIndex !== toSquare.fileIndex
        && !targetHadPiece
    );
}

function applyHistoryCastlingMove(position, movingPiece, fromSquare, toSquare, size) {
    if (movingPiece[1] !== 'K' || !fromSquare || !toSquare) return;
    if (Math.abs(fromSquare.fileIndex - toSquare.fileIndex) !== 2) return;

    const isKingSide = toSquare.fileIndex > fromSquare.fileIndex;
    const rookFromFile = isKingSide ? size - 1 : 0;
    const rookToFile = isKingSide ? fromSquare.fileIndex + 1 : fromSquare.fileIndex - 1;
    const rookFrom = historySquareFromParts(rookFromFile, fromSquare.rank);
    const rookTo = historySquareFromParts(rookToFile, fromSquare.rank);
    const rook = position[rookFrom];

    if (!rook) return;
    delete position[rookFrom];
    position[rookTo] = rook;
}

function frontendPromotionPiece(movingPiece, promotion) {
    if (!promotion || !movingPiece || movingPiece[1] !== 'P') return '';
    const typeMap = {
        queen: 'Q',
        rook: 'R',
        bishop: 'B',
        knight: 'N'
    };
    const type = typeMap[promotion];
    return type ? `${movingPiece[0]}${type}` : '';
}

function parseHistorySquare(square) {
    if (!square || square.length < 2) return null;
    const fileIndex = square.charCodeAt(0) - 'a'.charCodeAt(0);
    const rank = Number(square.slice(1));
    if (!Number.isInteger(rank) || fileIndex < 0 || rank < 1) return null;
    return { fileIndex, rank };
}

function historySquareFromParts(fileIndex, rank) {
    return `${fileLabel(fileIndex)}${rank}`;
}

function renderHistoryReplayPosition({ animate = false, previousIndex = null } = {}) {
    const state = historyReplayState;
    const host = document.getElementById('history-analysis-board');
    if (!state || !host) return;

    const position = state.positions[state.index] || {};

    if (state.size === 8) {
        renderHistoryClassicReplayBoard(host, state, position, animate);
    } else {
        renderHistoryCustomReplayBoard(host, state, position, animate ? previousIndex : null);
    }

    updateHistoryReplayControls();
    updateHistoryMoveSelection();
}

function renderHistoryClassicReplayBoard(host, state, position, animate) {
    let boardHost = document.getElementById('history-replay-board');
    if (!boardHost || !historyReplayBoard) {
        host.innerHTML = '';
        boardHost = document.createElement('div');
        boardHost.id = 'history-replay-board';
        boardHost.className = 'history-classic-board';
        host.appendChild(boardHost);

        historyReplayBoard = Chessboard('history-replay-board', {
            draggable: false,
            orientation: state.orientation,
            position,
            pieceTheme: piece => getVisualPieceSrc(piece, state.record?.visualState)
        });
    } else {
        historyReplayBoard.position(position, animate);
    }

    paintRenderedClassicSquares('#history-replay-board', state.record?.visualState);
    requestAnimationFrame(() => paintRenderedClassicSquares('#history-replay-board', state.record?.visualState));
}

function renderHistoryCustomReplayBoard(host, state, position, previousIndex = null) {
    if (historyReplayBoard) {
        historyReplayBoard.destroy();
        historyReplayBoard = null;
    }

    const animation = getHistoryReplayAnimation(previousIndex, state.index);
    const size = state.size;
    const visualState = state.record?.visualState || {};
    const lightSquare = getVisualSquareStrategy(visualState, 'light');
    const darkSquare = getVisualSquareStrategy(visualState, 'dark');
    const grid = document.createElement('div');
    grid.className = 'history-board-grid history-custom-board';
    grid.dataset.size = String(size);
    grid.dataset.orientation = state.orientation;
    grid.style.gridTemplateColumns = `repeat(${size}, minmax(0, 1fr))`;
    grid.style.gridTemplateRows = `repeat(${size}, minmax(0, 1fr))`;

    host.innerHTML = '';
    for (let row = 0; row < size; row += 1) {
        for (let col = 0; col < size; col += 1) {
            const square = document.createElement('span');
            const logicalRow = customLogicalIndex(row, size, state.orientation);
            const logicalCol = customLogicalIndex(col, size, state.orientation);
            const key = historySquareName(logicalRow, logicalCol, size);
            const strategy = (logicalRow + logicalCol) % 2 === 0 ? lightSquare : darkSquare;

            square.className = 'history-board-square';
            square.dataset.square = key;
            square.title = key;
            square.style.backgroundImage = `url("${strategy.getSrc()}")`;
            square.style.backgroundColor = strategy.getColor();

            appendHistoryNotation(square, row, col, logicalRow, logicalCol, size);

            const piece = position[key];
            if (piece) {
                const img = document.createElement('img');
                img.src = getVisualPieceSrc(piece, visualState);
                img.alt = '';
                square.appendChild(img);
            }
            grid.appendChild(square);
        }
    }

    host.dataset.meta = `${size}×${size}`;
    host.appendChild(grid);

    if (animation) {
        animateHistoryCustomPiece(host, animation);
    }
}

function getHistoryReplayAnimation(previousIndex, nextIndex) {
    const state = historyReplayState;
    if (!state || previousIndex === null || previousIndex === nextIndex) return null;
    if (Math.abs(nextIndex - previousIndex) !== 1) return null;

    const movingForward = nextIndex > previousIndex;
    const move = state.moves[movingForward ? previousIndex : nextIndex];
    if (!move?.from || !move?.to) return null;

    const from = movingForward ? move.from : move.to;
    const to = movingForward ? move.to : move.from;
    const piece = state.positions[nextIndex]?.[to] || backendPieceToFrontendCode(move.piece);
    return piece ? { from, to, piece } : null;
}

function animateHistoryCustomPiece(host, animation) {
    requestAnimationFrame(() => {
        const fromSquare = host.querySelector(`[data-square="${animation.from}"]`);
        const toSquare = host.querySelector(`[data-square="${animation.to}"]`);
        if (!fromSquare || !toSquare) return;

        const toImage = toSquare.querySelector('img');
        if (toImage) {
            toImage.style.visibility = 'hidden';
        }

        const fromRect = fromSquare.getBoundingClientRect();
        const toRect = toSquare.getBoundingClientRect();
        const inset = fromRect.width * 0.08;
        const clone = document.createElement('img');
        clone.className = 'history-moving-piece';
        clone.src = getVisualPieceSrc(animation.piece, historyReplayState?.record?.visualState);
        clone.alt = '';
        clone.style.left = `${fromRect.left + inset}px`;
        clone.style.top = `${fromRect.top + inset}px`;
        clone.style.width = `${fromRect.width - inset * 2}px`;
        clone.style.height = `${fromRect.height - inset * 2}px`;
        document.body.appendChild(clone);

        const animationHandle = clone.animate([
            { transform: 'translate3d(0, 0, 0)' },
            { transform: `translate3d(${toRect.left - fromRect.left}px, ${toRect.top - fromRect.top}px, 0)` }
        ], {
            duration: 240,
            easing: 'cubic-bezier(.2,.8,.2,1)'
        });

        animationHandle.onfinish = () => {
            clone.remove();
            if (toImage) {
                toImage.style.visibility = '';
            }
        };
        animationHandle.oncancel = animationHandle.onfinish;
    });
}

function moveHistoryReplay(action) {
    const state = historyReplayState;
    if (!state) return;

    const previousIndex = state.index;
    const maxIndex = Math.max(0, state.positions.length - 1);
    if (action === 'start') {
        state.index = 0;
    } else if (action === 'prev') {
        state.index = Math.max(0, state.index - 1);
    } else if (action === 'next') {
        state.index = Math.min(maxIndex, state.index + 1);
    } else if (action === 'end') {
        state.index = maxIndex;
    }

    if (state.index === previousIndex) {
        updateHistoryReplayControls();
        return;
    }

    renderHistoryReplayPosition({
        animate: Math.abs(state.index - previousIndex) === 1,
        previousIndex
    });
}

function updateHistoryReplayControls() {
    const state = historyReplayState;
    const maxIndex = state ? Math.max(0, state.positions.length - 1) : 0;
    const index = state ? state.index : 0;
    const label = document.getElementById('history-replay-step');
    if (label) {
        label.textContent = index === 0
            ? `Start position · 0 / ${maxIndex}`
            : `Move ${index} / ${maxIndex}`;
    }

    document.querySelectorAll('[data-history-replay]').forEach(button => {
        const action = button.dataset.historyReplay;
        button.disabled = !state
            || (action === 'start' && index === 0)
            || (action === 'prev' && index === 0)
            || (action === 'next' && index === maxIndex)
            || (action === 'end' && index === maxIndex);
    });
}

function updateHistoryMoveSelection() {
    document.querySelectorAll('[data-history-move-index]').forEach(row => {
        const moveIndex = Number(row.dataset.historyMoveIndex);
        row.classList.toggle('active', Boolean(historyReplayState && historyReplayState.index === moveIndex));
    });
}

function destroyHistoryReplayBoard() {
    if (historyReplayBoard) {
        historyReplayBoard.destroy();
        historyReplayBoard = null;
    }
    historyReplayState = null;
    updateHistoryReplayControls();
}

function renderHistoryMoveList(record, emptyMessage = 'No moves saved for this game.') {
    const list = document.getElementById('history-move-list');
    if (!list) return;

    list.innerHTML = '';
    const moves = Array.isArray(record?.moves) ? record.moves : [];
    if (moves.length === 0) {
        const empty = document.createElement('div');
        empty.className = 'history-empty history-moves-empty';
        empty.textContent = emptyMessage;
        list.appendChild(empty);
        return;
    }

    moves.forEach((move, index) => {
        const row = document.createElement('div');
        row.className = 'history-move-row';
        row.dataset.historyMoveIndex = String(index + 1);
        row.addEventListener('click', () => {
            if (!historyReplayState) return;
            const previousIndex = historyReplayState.index;
            historyReplayState.index = index + 1;
            renderHistoryReplayPosition({
                animate: Math.abs(historyReplayState.index - previousIndex) === 1,
                previousIndex
            });
        });

        const number = document.createElement('span');
        number.className = 'history-move-number';
        number.textContent = String(index + 1);

        const text = document.createElement('strong');
        text.className = 'history-move-text';
        text.textContent = formatHistoryMove(move);
        text.title = text.textContent;

        row.append(number, text);
        list.appendChild(row);
    });

    updateHistoryMoveSelection();
}

function resultLabel(result) {
    if (result === 'win') return 'Win';
    if (result === 'loss') return 'Loss';
    if (result === 'draw') return 'Draw';
    if (result === 'active') return 'Active';
    if (result === 'abandoned') return 'Expired';
    return 'Unknown';
}

function formatHistoryDate(timestamp) {
    if (!timestamp) return '-';
    return new Date(timestamp).toLocaleDateString('en-US', {
        month: 'short',
        day: 'numeric',
        hour: '2-digit',
        minute: '2-digit'
    });
}

function normalizeHistoryGames(games = []) {
    if (!Array.isArray(games)) return [];
    return games.map(normalizeHistoryGame).filter(record => record.id);
}

function normalizeHistoryGame(game = {}) {
    const boardState = normalizeHistoryBoardState(game.board_state);
    const visualState = normalizeHistoryVisualState(game.visual_state);
    const boardSize = clampHistoryBoardSize(game.board_size || boardState.board_size || boardState.board?.width || boardState.board?.height || 8);
    const timeLimitMs = Number(game.time_limit_ms || 0);
    const opponent = game.opponent?.username || game.opponent?.id || fallbackOpponentName(game);
    const timestamp = game.created_at || game.updated_at || '';

    return {
        id: game.id || '',
        mode: game.mode || modeForBoardSize(boardSize),
        boardSize,
        timeLimitMs,
        timeControl: formatTimeControl(timeLimitMs),
        isRanked: Boolean(game.is_ranked),
        status: game.status || boardState.status || 'unknown',
        turn: game.turn || boardState.turn || '',
        playerColor: game.player_color || '',
        result: normalizeHistoryResult(game.result, game.status || boardState.status, game.player_color),
        opponent,
        white: game.white || null,
        black: game.black || null,
        winnerId: game.winner_id || null,
        timestamp,
        boardState,
        visualState,
        moves: normalizeHistoryMoves(boardState.moves)
    };
}

function normalizeHistoryBoardState(boardState) {
    if (!boardState) return {};
    if (typeof boardState === 'string') {
        try {
            return JSON.parse(boardState);
        } catch {
            return {};
        }
    }
    if (typeof boardState === 'object') {
        return boardState;
    }
    return {};
}

function normalizeHistoryVisualState(visualState) {
    if (!visualState) return {};
    if (typeof visualState === 'string') {
        try {
            const parsed = JSON.parse(visualState);
            return parsed && typeof parsed === 'object' && !Array.isArray(parsed) ? parsed : {};
        } catch {
            return {};
        }
    }
    if (typeof visualState === 'object' && !Array.isArray(visualState)) {
        return visualState;
    }
    return {};
}

function normalizeHistoryMoves(moves) {
    if (!Array.isArray(moves)) return [];
    return moves
        .filter(move => move?.from && move?.to)
        .map(move => ({
            from: move.from,
            to: move.to,
            piece: move.piece || null,
            captured: move.captured || null,
            promotion: move.promotion || ''
        }));
}

function normalizeHistoryResult(result, status = '', playerColor = '') {
    if (result === 'win' || result === 'loss' || result === 'draw' || result === 'active' || result === 'abandoned') {
        return result;
    }

    const normalizedStatus = String(status || '');
    if (normalizedStatus === 'active') return 'active';
    if (normalizedStatus === 'abandoned') return 'abandoned';
    if (normalizedStatus.includes('draw')) return 'draw';

    const whiteWon = normalizedStatus.startsWith('white_won');
    const blackWon = normalizedStatus.startsWith('black_won');
    if (!whiteWon && !blackWon) return 'unknown';

    if (!playerColor) return 'unknown';
    return (whiteWon && playerColor === 'white') || (blackWon && playerColor === 'black') ? 'win' : 'loss';
}

function updateHistoryRecord(detail) {
    if (!detail?.id) return;
    const index = historyRecords.findIndex(record => record.id === detail.id);
    if (index >= 0) {
        historyRecords[index] = { ...historyRecords[index], ...detail };
    } else {
        historyRecords.unshift(detail);
    }
}

function fallbackOpponentName(game) {
    if (game.player_color === 'black') return game.white?.username || game.white?.id || 'White';
    return game.black?.username || game.black?.id || 'Black';
}

function formatTimeControl(timeLimitMs) {
    const ms = Number(timeLimitMs || 0);
    if (ms <= 0) return 'unknown time';
    const minutes = ms / 60000;
    if (Number.isInteger(minutes)) return `${minutes} min`;
    return `${Math.round(ms / 1000)} sec`;
}

function historyFormatLabel(record) {
    return `${modeLabel(record.mode, record.boardSize)} · ${record.timeControl}`;
}

function historyStatusLabel(record) {
    const ranked = record.isRanked ? 'ranked' : 'casual';
    return `${record.status || 'unknown'} · ${ranked}`;
}

function backendPiecesToHistoryPosition(pieces) {
    if (!Array.isArray(pieces)) return {};
    return pieces.reduce((position, piece) => {
        const code = backendPieceToFrontendCode(piece);
        if (code && piece.square) {
            position[piece.square] = code;
        }
        return position;
    }, {});
}

function historySquareName(row, col, size) {
    return `${fileLabel(col)}${size - row}`;
}

function appendHistoryNotation(square, visualRow, visualCol, logicalRow, logicalCol, size) {
    if (visualCol === 0) {
        const rank = document.createElement('span');
        rank.className = 'history-notation history-numeric';
        rank.textContent = String(size - logicalRow);
        square.appendChild(rank);
    }

    if (visualRow === size - 1) {
        const file = document.createElement('span');
        file.className = 'history-notation history-alpha';
        file.textContent = fileLabel(logicalCol);
        square.appendChild(file);
    }
}

function clampHistoryBoardSize(size) {
    const parsed = Number(size);
    if (parsed === 10 || parsed === 12) return parsed;
    return 8;
}

function formatHistoryMove(move) {
    const color = move.piece?.color ? `${capitalize(move.piece.color)} ` : '';
    const piece = move.piece?.type || 'piece';
    const capture = move.captured
        ? ` captures ${move.captured.color || ''} ${move.captured.type || 'piece'}`.replace(/\s+/g, ' ').trimEnd()
        : '';
    const promotion = move.promotion ? ` promotes to ${move.promotion}` : '';
    return `${color}${piece} ${move.from} -> ${move.to}${capture}${promotion}`;
}

function capitalize(value) {
    const text = String(value || '');
    return text ? text.charAt(0).toUpperCase() + text.slice(1) : '';
}

function renderEmojiMessages() {
    const log = document.getElementById('emoji-chat-log');
    if (!log) return;

    log.innerHTML = '';
    emojiMessages.forEach(message => {
        const row = document.createElement('div');
        row.className = `emoji-message ${message.sender}`;

        if (message.text) {
            const text = document.createElement('span');
            text.className = 'emoji-message-text';
            text.textContent = message.text;
            row.appendChild(text);
            log.appendChild(row);
            return;
        }

        if (message.src) {
            const name = document.createElement('span');
            name.className = 'emoji-message-name';
            name.textContent = message.name;
            row.appendChild(name);

            const icon = document.createElement('span');
            icon.className = 'emoji-message-icon';
            const img = document.createElement('img');
            img.src = message.src;
            img.alt = message.label;
            icon.appendChild(img);
            row.appendChild(icon);
        }
        log.appendChild(row);
    });

    log.scrollTop = log.scrollHeight;
}

function appendSystemEmojiMessage(text) {
    if (!text) return;
    emojiMessages.push({
        id: createUserId('chat-system'),
        sender: 'system',
        text
    });
    renderEmojiMessages();
}

async function loadSecurityConfig() {
    if (!window.ChessApi?.config) return;

    try {
        const config = await ChessApi.config();
        securityConfig = normalizeSecurityConfig(config);
        renderTurnstileWidget();
    } catch (error) {
        securityConfig = normalizeSecurityConfig(null);
        console.warn('Unable to load security config', error);
    }
}

function normalizeSecurityConfig(config) {
    const turnstile = config?.turnstile || {};
    return {
        turnstile: {
            enabled: Boolean(turnstile.enabled),
            siteKey: String(turnstile.site_key || turnstile.siteKey || '').trim()
        }
    };
}

function renderTurnstileWidget() {
    const wrapper = document.getElementById('account-turnstile');
    const host = document.getElementById('account-turnstile-widget');
    if (!wrapper || !host) return;

    const enabled = Boolean(securityConfig.turnstile.enabled && securityConfig.turnstile.siteKey);
    wrapper.classList.toggle('hidden', !enabled);
    if (!enabled) {
        turnstileToken = '';
        host.innerHTML = '';
        turnstileWidgetId = null;
        return;
    }

    loadTurnstileScript()
        .then(() => {
            if (!window.turnstile || turnstileWidgetId !== null) return;
            turnstileWidgetId = window.turnstile.render(host, {
                sitekey: securityConfig.turnstile.siteKey,
                callback: token => {
                    turnstileToken = token || '';
                },
                'expired-callback': () => {
                    turnstileToken = '';
                    resetTurnstileWidget();
                },
                'error-callback': () => {
                    turnstileToken = '';
                }
            });
        })
        .catch(error => {
            turnstileToken = '';
            showAccountMessage('Human verification could not load. Try again.');
            console.warn('Unable to load Turnstile', error);
        });
}

function loadTurnstileScript() {
    if (window.turnstile) {
        return Promise.resolve();
    }
    if (turnstileScriptPromise) {
        return turnstileScriptPromise;
    }

    turnstileScriptPromise = new Promise((resolve, reject) => {
        const script = document.createElement('script');
        script.src = 'https://challenges.cloudflare.com/turnstile/v0/api.js?render=explicit';
        script.async = true;
        script.defer = true;
        script.onload = () => resolve();
        script.onerror = () => reject(new Error('Turnstile script failed to load'));
        document.head.appendChild(script);
    });

    return turnstileScriptPromise;
}

function resetTurnstileWidget() {
    if (window.turnstile && turnstileWidgetId !== null) {
        window.turnstile.reset(turnstileWidgetId);
    }
}

function currentTurnstileToken() {
    if (!securityConfig.turnstile.enabled) {
        return '';
    }
    return turnstileToken || '';
}

function bindAccountForm() {
    const form = document.getElementById('account-form');
    const loginForm = document.getElementById('account-login-form');
    const verifyForm = document.getElementById('account-verify-form');
    const resendCodeButton = document.getElementById('account-resend-code-btn');
    const showSignupButton = document.getElementById('account-show-signup-btn');
    const showLoginButton = document.getElementById('account-show-login-btn');
    const verifyBackLoginButton = document.getElementById('account-verify-back-login-btn');
    const profileAvatarInput = document.getElementById('account-profile-avatar-input');
    const refreshButton = document.getElementById('account-refresh-btn');
    const logoutButton = document.getElementById('account-logout-btn');

    showSignupButton?.addEventListener('click', () => {
        pendingVerificationEmail = '';
        clearAccountVerificationTimer();
        setAccountAuthMode('signup');
        showAccountMessage('');
    });

    showLoginButton?.addEventListener('click', () => {
        clearAccountVerificationTimer();
        setAccountAuthMode('login');
        showAccountMessage('');
    });

    verifyBackLoginButton?.addEventListener('click', () => {
        pendingVerificationEmail = '';
        clearAccountVerificationTimer();
        setAccountAuthMode('login');
        showAccountMessage('');
    });

    form?.addEventListener('submit', async event => {
        event.preventDefault();
        const username = document.getElementById('account-username')?.value.trim() || '';
        const email = document.getElementById('account-email')?.value.trim() || '';
        const passwordInput = document.getElementById('account-password');
        const password = passwordInput?.value || '';

        if (!username || !email || !password) {
            showAccountMessage('Username, email and password are required.');
            return;
        }

        if (securityConfig.turnstile.enabled && !securityConfig.turnstile.siteKey) {
            showAccountMessage('Human verification is not configured. Contact server admin.');
            return;
        }

        const turnstileVerificationToken = currentTurnstileToken();
        if (securityConfig.turnstile.enabled && !turnstileVerificationToken) {
            showAccountMessage('Complete human verification before registering.');
            return;
        }

        try {
            setAccountFormsBusy(true);
            showAccountMessage('Creating account...');
            await ChessApi.register({
                username,
                email,
                password,
                turnstileToken: turnstileVerificationToken
            });

            if (passwordInput) passwordInput.value = '';
            const loginEmail = document.getElementById('account-login-email');
            if (loginEmail) loginEmail.value = email;
            const verifyEmail = document.getElementById('account-verify-email');
            if (verifyEmail) verifyEmail.value = email;

            pendingVerificationEmail = email;
            setAccountAuthMode('verify', { render: false });
            startAccountVerificationTimer();
            accountEditing = false;
            renderAccountProfile();
            showAccountMessage('Account created. Enter the 6-digit code from your email within 1 minute.');
        } catch (error) {
            showAccountMessage(getAccountErrorMessage(error));
            resetTurnstileWidget();
        } finally {
            turnstileToken = '';
            setAccountFormsBusy(false);
        }
    });

    loginForm?.addEventListener('submit', async event => {
        event.preventDefault();
        const email = document.getElementById('account-login-email')?.value.trim() || '';
        const passwordInput = document.getElementById('account-login-password');
        const password = passwordInput?.value || '';

        if (!email || !password) {
            showAccountMessage('Email and password are required.');
            return;
        }

        try {
            setAccountFormsBusy(true);
            showAccountMessage('Logging in...');
            await ChessApi.login({ email, password });
            const profile = await ChessApi.me();

            if (passwordInput) passwordInput.value = '';
            applyBackendAccountProfile(profile);
            resetHistoryState();
            clearAccountVerificationTimer();
            accountEditing = false;
            renderAccountProfile();
            showAccountMessage('Logged in.');
        } catch (error) {
            if (error?.status === 401) {
                ChessApi.clearToken();
                resetHistoryState();
                accountProfile = createEmptyAccountProfile();
                renderAccountProfile();
            }
            if (error?.status === 403) {
                pendingVerificationEmail = email;
                setAccountAuthMode('verify', { render: false });
                startAccountVerificationTimer();
                renderAccountProfile();
            }
            showAccountMessage(getAccountErrorMessage(error));
        } finally {
            setAccountFormsBusy(false);
        }
    });

    verifyForm?.addEventListener('submit', async event => {
        event.preventDefault();
        const email = pendingVerificationEmail || document.getElementById('account-verify-email')?.value.trim() || '';
        const codeInput = document.getElementById('account-verify-code');
        const code = codeInput?.value.trim() || '';

        if (!email || !code) {
            showAccountMessage('Email and verification code are required.');
            return;
        }

        try {
            setAccountFormsBusy(true);
            showAccountMessage('Verifying email...');
            await ChessApi.verifyEmail({ email, code });

            const loginEmail = document.getElementById('account-login-email');
            if (loginEmail) loginEmail.value = email;
            if (codeInput) codeInput.value = '';
            pendingVerificationEmail = '';
            clearAccountVerificationTimer();
            setAccountAuthMode('login', { render: false });
            renderAccountProfile();
            showAccountMessage('Email verified. You can log in now.');
        } catch (error) {
            showAccountMessage(getAccountErrorMessage(error));
            if (error?.status === 410) {
                pendingVerificationEmail = '';
                clearAccountVerificationTimer();
                setAccountAuthMode('login', { render: false });
                renderAccountProfile();
            }
        } finally {
            setAccountFormsBusy(false);
        }
    });

    resendCodeButton?.addEventListener('click', async () => {
        const email = pendingVerificationEmail
            || document.getElementById('account-verify-email')?.value.trim()
            || document.getElementById('account-email')?.value.trim()
            || document.getElementById('account-login-email')?.value.trim()
            || '';

        if (!email) {
            showAccountMessage('Enter your email to resend the verification code.');
            return;
        }

        try {
            setAccountFormsBusy(true);
            showAccountMessage('Sending verification code...');
            await ChessApi.resendVerification({ email });
            const verifyEmail = document.getElementById('account-verify-email');
            if (verifyEmail) verifyEmail.value = email;
            pendingVerificationEmail = email;
            setAccountAuthMode('verify', { render: false });
            startAccountVerificationTimer();
            renderAccountProfile();
            showAccountMessage('If this email is registered and not verified, a new code was sent. You have 1 minute.');
        } catch (error) {
            showAccountMessage(getAccountErrorMessage(error));
        } finally {
            setAccountFormsBusy(false);
        }
    });

    profileAvatarInput?.addEventListener('change', async event => {
        const [file] = Array.from(event.target.files || []);
        if (!file || !accountProfile.signedIn) return;

        try {
            accountProfile.avatarSrc = await readAccountAvatarAsDataUrl(file);
            renderAccountProfile();
            showAccountMessage('Profile image applied for this browser session.');
        } catch (error) {
            showAccountMessage(error.message);
        } finally {
            event.target.value = '';
        }
    });

    refreshButton?.addEventListener('click', async () => {
        await refreshAccountFromBackend({ showMessages: true });
    });

    logoutButton?.addEventListener('click', () => {
        if (queuedForMatch) {
            window.ChessSocket?.cancelQueue?.();
        }
        if (activeRemoteGame && currentGameState?.status === 'active' && window.ChessSocket?.isOpen?.()) {
            try {
                window.ChessSocket?.leaveGame?.();
            } catch (error) {
                console.warn('Unable to notify backend about leaving game', error);
            }
        }
        window.ChessSocket?.close?.();
        ChessApi.logout();
        clearLegacyAccountProfile();
        resetHistoryState();
        accountProfile = createEmptyAccountProfile();
        activeMatchRequest = null;
        queuedForMatch = false;
        activeRemoteGame = false;
        currentGameState = null;
        currentPlayerColor = null;
        currentGameId = null;
        currentValidMoves = {};
        pendingClassicMove = null;
        classicSnapbackInProgress = false;
        queuedClassicPositionUpdate = null;
        clearClassicMoveHighlights();
        hideGameFinishedOverlay();
        resetDrawOfferState();
        resetNetworkWarning();
        resetRatingState();
        accountEditing = false;
        pendingVerificationEmail = '';
        clearAccountVerificationTimer();
        accountAuthMode = 'login';
        renderAccountProfile();
        showAccountMessage('Logged out.');
    });
}

function loadAccountProfile() {
    const profile = createEmptyAccountProfile();
    const hasToken = Boolean(window.ChessApi?.hasToken?.());
    profile.registered = hasToken;
    profile.signedIn = hasToken;
    return profile;
}

function renderAccountProfile() {
    const username = document.getElementById('account-username');
    const email = document.getElementById('account-email');
    const password = document.getElementById('account-password');
    const loginPassword = document.getElementById('account-login-password');
    const verifyEmail = document.getElementById('account-verify-email');
    const verifyCode = document.getElementById('account-verify-code');
    const chip = document.getElementById('account-chip');
    const authPanel = document.getElementById('account-auth-panel');
    const loginForm = document.getElementById('account-login-form');
    const registerForm = document.getElementById('account-form');
    const verifyForm = document.getElementById('account-verify-form');
    const accountFormTitle = document.getElementById('account-form-title');
    const accountSubmitButton = document.getElementById('account-submit-btn');
    const profilePanel = document.getElementById('account-profile-panel');
    const profileName = document.getElementById('account-profile-name');
    const profileEmail = document.getElementById('account-profile-email');
    const profileEmailStatus = document.getElementById('account-profile-email-status');

    const shouldShowProfile = accountProfile.signedIn;

    if (shouldShowProfile) {
        pendingVerificationEmail = '';
        if (username) username.value = '';
        if (email) email.value = '';
        if (password) password.value = '';
        if (loginPassword) loginPassword.value = '';
        if (verifyCode) verifyCode.value = '';
    }
    if (accountFormTitle) accountFormTitle.textContent = 'Register';
    if (accountSubmitButton) accountSubmitButton.textContent = 'Register';
    if (chip) {
        const displayName = accountProfile.username || 'Profile';
        const label = accountProfile.signedIn ? `Account: ${displayName}` : 'Account';
        chip.title = label;
        chip.setAttribute('aria-label', label);
    }

    authPanel?.classList.toggle('hidden', shouldShowProfile);
    loginForm?.classList.toggle('hidden', shouldShowProfile || accountAuthMode !== 'login');
    registerForm?.classList.toggle('hidden', shouldShowProfile || accountAuthMode !== 'signup');
    verifyForm?.classList.toggle('hidden', shouldShowProfile || accountAuthMode !== 'verify');
    profilePanel?.classList.toggle('hidden', !shouldShowProfile);

    if (verifyEmail) verifyEmail.value = pendingVerificationEmail;
    renderAccountVerificationCountdown();

    renderAccountAvatar('account-profile-avatar', 'account-profile-avatar-fallback', accountProfile.avatarSrc, accountInitial());

    if (profileName) profileName.textContent = accountProfile.username || 'Player';
    if (profileEmail) profileEmail.textContent = accountProfile.email || '-';
    if (profileEmailStatus) profileEmailStatus.textContent = accountProfile.emailVerified ? 'Verified' : 'Not verified';
    renderAccountRatings();
}

function startAccountVerificationTimer() {
    clearAccountVerificationTimer();
    accountVerificationDeadlineMs = Date.now() + ACCOUNT_VERIFICATION_SECONDS * 1000;
    renderAccountVerificationCountdown();
    accountVerificationTimerId = window.setInterval(renderAccountVerificationCountdown, 1000);
}

function clearAccountVerificationTimer() {
    if (accountVerificationTimerId) {
        window.clearInterval(accountVerificationTimerId);
        accountVerificationTimerId = null;
    }
    accountVerificationDeadlineMs = 0;
    renderAccountVerificationCountdown();
}

function renderAccountVerificationCountdown() {
    const countdown = document.getElementById('account-verify-countdown');
    if (!countdown) return;

    if (accountAuthMode !== 'verify' || !accountVerificationDeadlineMs) {
        countdown.textContent = '';
        return;
    }

    const remainingSeconds = Math.max(0, Math.ceil((accountVerificationDeadlineMs - Date.now()) / 1000));
    if (remainingSeconds <= 0) {
        clearAccountVerificationTimer();
        pendingVerificationEmail = '';
        setAccountAuthMode('login', { render: false });
        renderAccountProfile();
        showAccountMessage('Verification time expired. Log in again or request a new code.');
        return;
    }

    countdown.textContent = `Confirm your account within ${remainingSeconds}s.`;
}

function setAccountAuthMode(mode, { render = true } = {}) {
    accountAuthMode = ['login', 'signup', 'verify'].includes(mode) ? mode : 'login';
    if (render) {
        renderAccountProfile();
    }
}

function createEmptyAccountProfile() {
    return {
        id: '',
        username: '',
        email: '',
        avatarSrc: '',
        rating: '-',
        ratings: [],
        emailVerified: false,
        registered: false,
        signedIn: false
    };
}

function applyBackendAccountProfile(profile) {
    const ratings = normalizeRatingList(profile?.ratings);
    accountProfile = {
        id: profile?.id || '',
        username: profile?.username || '',
        email: profile?.email || '',
        avatarSrc: accountProfile.avatarSrc || '',
        rating: profile?.rating ?? '-',
        ratings,
        emailVerified: Boolean(profile?.email_verified ?? profile?.emailVerified),
        registered: true,
        signedIn: true
    };
}

function normalizeRatingList(ratings = []) {
    if (!Array.isArray(ratings)) return [];
    return ratings
        .map(rating => ({
            mode: rating.mode || modeForBoardSize(Number(rating.board_size || rating.boardSize || 8)),
            boardSize: Number(rating.board_size || rating.boardSize || 8),
            timeLimitMs: Number(rating.time_limit_ms || rating.timeLimitMs || 0),
            timeLimitMinutes: Number(rating.time_limit_minutes || rating.timeLimitMinutes || 0),
            rating: Number(rating.rating ?? 1200),
            gamesPlayed: Number(rating.games_played || rating.gamesPlayed || 0)
        }))
        .filter(rating => rating.boardSize && rating.timeLimitMinutes);
}

function renderAccountRatings() {
    const list = document.getElementById('account-ratings-list');
    if (!list) return;

    list.innerHTML = '';
    if (!accountProfile.signedIn) return;

    const ratings = [...accountProfile.ratings].sort((a, b) => {
        if (a.boardSize !== b.boardSize) return a.boardSize - b.boardSize;
        return a.timeLimitMinutes - b.timeLimitMinutes;
    });

    if (ratings.length === 0) {
        const row = document.createElement('div');
        row.className = 'account-rating-row';
        row.append(createTextSpan('Scoped ratings are not loaded yet.'), createTextStrong('-'));
        list.appendChild(row);
        return;
    }

    ratings.forEach(rating => {
        const row = document.createElement('div');
        row.className = 'account-rating-row';

        const label = `${modeLabel(rating.mode, rating.boardSize)} · ${rating.timeLimitMinutes} min · ${rating.gamesPlayed} games`;
        row.append(createTextSpan(label), createTextStrong(String(rating.rating)));
        list.appendChild(row);
    });
}

async function refreshAccountFromBackend({ showMessages = false } = {}) {
    if (!window.ChessApi?.hasToken?.()) {
        resetHistoryState();
        accountProfile = createEmptyAccountProfile();
        clearAccountVerificationTimer();
        renderAccountProfile();
        return false;
    }

    try {
        if (showMessages) {
            showAccountMessage('Refreshing profile...');
        }

        const profile = await ChessApi.me();
        applyBackendAccountProfile(profile);
        renderAccountProfile();

        if (showMessages) {
            showAccountMessage('Profile refreshed.');
        }
        return true;
    } catch (error) {
        if (error?.status === 401) {
            ChessApi.clearToken();
            resetHistoryState();
            accountProfile = createEmptyAccountProfile();
            clearAccountVerificationTimer();
            renderAccountProfile();
            showAccountMessage(showMessages ? 'Session expired. Log in again.' : '');
            return false;
        }

        if (showMessages || document.getElementById('page-account')?.classList.contains('active')) {
            showAccountMessage(getAccountErrorMessage(error));
        }
        return false;
    }
}

function setAccountFormsBusy(isBusy) {
    document.querySelectorAll('#account-auth-panel input, #account-auth-panel button, #account-profile-panel button')
        .forEach(element => {
            element.disabled = isBusy;
        });
}

function clearLegacyAccountProfile() {
    try {
        localStorage.removeItem(LEGACY_ACCOUNT_PROFILE_KEY);
    } catch {
        // Ignore storage errors; this is only a cleanup for the old local auth profile.
    }
}

function getAccountErrorMessage(error) {
    return window.ChessApi?.getErrorMessage?.(error) || playerFacingErrorMessage(error, 'Something went wrong with your account.');
}

function renderAccountAvatar(imageId, fallbackId, src, fallbackText) {
    const img = document.getElementById(imageId);
    const fallback = document.getElementById(fallbackId);

    if (img) {
        if (src) {
            img.src = src;
        } else {
            img.removeAttribute('src');
        }
        img.classList.toggle('hidden', !src);
    }

    if (fallback) {
        fallback.textContent = fallbackText;
        fallback.classList.toggle('hidden', Boolean(src));
    }
}

function accountInitial() {
    return (accountProfile.username || '?').trim().charAt(0).toUpperCase() || '?';
}

function showAccountMessage(message) {
    const messageEl = document.getElementById('account-message');
    if (messageEl) {
        messageEl.textContent = message;
    }
}

function pieceTheme(piece) {
    return getPieceSrc(piece);
}

function handleClassicDragStart(source, piece) {
    clearClassicMoveHighlights();

    if (!isClassicBackendGameReady()) {
        setMatchmakingStatus(GAME_SETUP_PENDING_MESSAGE);
        return false;
    }

    if (pendingClassicMove) {
        setMatchmakingStatus('Waiting for move confirmation.');
        return false;
    }

    if (classicSnapbackInProgress || queuedClassicPositionUpdate) {
        setMatchmakingStatus('Applying confirmed move.');
        return false;
    }

    if (currentGameState.status !== 'active') {
        setMatchmakingStatus(currentGameState.status || 'Game is not active.');
        return false;
    }

    if (!isCurrentPlayerTurn()) {
        setMatchmakingStatus('Opponent turn.');
        return false;
    }

    if (!isClassicPieceOwnedByPlayer(piece)) {
        setMatchmakingStatus('You can move only your pieces.');
        return false;
    }

    if (validTargetsForSquare(source).length === 0) {
        setMatchmakingStatus('This piece has no legal moves.');
        return false;
    }

    showClassicMoveHighlights(source, piece);
    return true;
}

function handleClassicDrop(source, target, piece) {
    clearClassicMoveHighlights();

    if (!target || target === 'offboard' || source === target) return 'snapback';

    if (!canSubmitClassicMove(source, target, piece)) {
        return 'snapback';
    }

    if (isPromotionMove(piece, target, currentVisualBoardSize)) {
        showPromotionPicker({ from: source, to: target, piece, boardSize: currentVisualBoardSize });
        return 'snapback';
    }

    submitBackendMove({ from: source, to: target });
    return 'snapback';
}

function handleClassicSnapbackEnd() {
    classicSnapbackInProgress = false;
    clearClassicMoveHighlights();
    flushQueuedClassicPositionUpdate();
}

function canSubmitClassicMove(source, target, piece) {
    if (!isClassicBackendGameReady()) {
        setMatchmakingStatus(GAME_SETUP_PENDING_MESSAGE);
        return false;
    }

    if (pendingClassicMove) {
        setMatchmakingStatus('Waiting for move confirmation.');
        return false;
    }

    if (classicSnapbackInProgress || queuedClassicPositionUpdate) {
        setMatchmakingStatus('Applying confirmed move.');
        return false;
    }

    if (!window.ChessSocket?.isOpen?.()) {
        setMatchmakingStatus(GAME_CONNECTION_LOST_MESSAGE);
        return false;
    }

    if (currentGameState.status !== 'active') {
        setMatchmakingStatus(currentGameState.status || 'Game is not active.');
        return false;
    }

    if (!isCurrentPlayerTurn()) {
        setMatchmakingStatus('Opponent turn.');
        return false;
    }

    if (!isClassicPieceOwnedByPlayer(piece)) {
        setMatchmakingStatus('You can move only your pieces.');
        return false;
    }

    if (!validTargetsForSquare(source).includes(target)) {
        setMatchmakingStatus('Illegal move.');
        return false;
    }

    return true;
}

function submitBackendMove({ from, to, promotion = '', snapback = true }) {
    pendingClassicMove = { from, to, promotion };
    setMatchmakingStatus(`Sending move ${from}-${to}${promotion ? `=${promotion}` : ''}...`);

    try {
        if (window.ChessSocket?.move) {
            ChessSocket.move({ from, to, promotion });
        } else {
            const payload = { from, to };
            if (promotion) {
                payload.promotion = promotion;
            }
            ChessSocket.send('MOVE', payload);
        }
        classicSnapbackInProgress = Boolean(snapback);
        return true;
    } catch (error) {
        pendingClassicMove = null;
        classicSnapbackInProgress = false;
        queuedClassicPositionUpdate = null;
        setMatchmakingStatus(playerFacingErrorMessage(error, 'Could not send the move.'));
        return false;
    }
}

function isPromotionMove(piece, targetSquare, boardSize = currentVisualBoardSize) {
    if (!piece || piece[1] !== 'P' || !targetSquare || !boardSize) return false;
    const rank = Number(String(targetSquare).slice(1));
    if (!Number.isInteger(rank)) return false;
    return (piece[0] === 'w' && rank === boardSize) || (piece[0] === 'b' && rank === 1);
}

function showPromotionPicker({ from, to, piece, boardSize }) {
    pendingPromotionMove = { from, to, piece, boardSize };
    renderPromotionPicker();
    setMatchmakingStatus('Choose promotion piece.');
}

function hidePromotionPicker() {
    const picker = document.getElementById('promotion-picker');
    if (!picker) return;
    picker.classList.add('hidden');
    picker.innerHTML = '';
}

function renderPromotionPicker() {
    const picker = document.getElementById('promotion-picker');
    if (!picker || !pendingPromotionMove) return;

    picker.innerHTML = '';
    picker.classList.remove('hidden');

    const title = document.createElement('div');
    title.className = 'promotion-picker-title';
    title.textContent = 'Promote pawn to';

    const options = document.createElement('div');
    options.className = 'promotion-options';

    promotionOptions().forEach(option => {
        const button = document.createElement('button');
        button.type = 'button';
        button.className = 'promotion-option';
        button.title = option.label;
        button.setAttribute('aria-label', option.label);

        const img = document.createElement('img');
        img.src = getPieceSrc(`${pendingPromotionMove.piece[0]}${option.code}`);
        img.alt = option.label;
        button.appendChild(img);

        button.addEventListener('click', () => choosePromotion(option.type));
        options.appendChild(button);
    });

    const cancel = document.createElement('button');
    cancel.type = 'button';
    cancel.className = 'promotion-cancel-btn';
    cancel.textContent = 'Cancel';
    cancel.addEventListener('click', () => {
        pendingPromotionMove = null;
        hidePromotionPicker();
        setMatchmakingStatus('Promotion cancelled.');
    });

    picker.append(title, options, cancel);
}

function promotionOptions() {
    return [
        { code: 'Q', type: 'queen', label: 'Queen' },
        { code: 'R', type: 'rook', label: 'Rook' },
        { code: 'B', type: 'bishop', label: 'Bishop' },
        { code: 'N', type: 'knight', label: 'Knight' }
    ];
}

function choosePromotion(promotion) {
    if (!pendingPromotionMove) return;
    const move = pendingPromotionMove;
    pendingPromotionMove = null;
    hidePromotionPicker();
    submitBackendMove({
        from: move.from,
        to: move.to,
        promotion,
        snapback: move.boardSize === 8
    });
}

function isClassicBackendGameReady() {
    return activeRemoteGame
        && currentGameState
        && currentPlayerColor
        && boardSizeFromGameState(currentGameState) === 8;
}

function isCurrentPlayerTurn() {
    return currentGameState?.turn === currentPlayerColor;
}

function isClassicPieceOwnedByPlayer(piece) {
    if (!piece || !currentPlayerColor) return false;
    return piece[0] === frontendColorCode(currentPlayerColor);
}

function frontendColorCode(color) {
    return color === 'white' ? 'w' : 'b';
}

function validTargetsForSquare(square) {
    return Array.isArray(currentValidMoves?.[square]) ? currentValidMoves[square] : [];
}

function showClassicMoveHighlights(source, piece) {
    const sourceSquare = classicSquareElement(source);
    sourceSquare?.classList.add('classic-move-source');

    validTargetsForSquare(source).forEach(target => {
        const targetSquare = classicSquareElement(target);
        if (!targetSquare) return;

        const targetClass = isClassicCaptureTarget(piece, target)
            ? 'classic-capture-target'
            : 'classic-move-target';
        targetSquare.classList.add(targetClass);
    });
}

function clearClassicMoveHighlights() {
    document
        .querySelectorAll('#myBoard .classic-move-source, #myBoard .classic-move-target, #myBoard .classic-capture-target')
        .forEach(square => {
            square.classList.remove('classic-move-source', 'classic-move-target', 'classic-capture-target');
        });
}

function classicSquareElement(square) {
    if (!square) return null;
    return document.querySelector(`#myBoard .square-${square}`);
}

function isClassicCaptureTarget(piece, target) {
    const targetPiece = board?.position?.()?.[target];
    return Boolean(piece && targetPiece && targetPiece[0] !== piece[0]);
}

function trackCapture(movingPiece, capturedPiece) {
    if (!movingPiece || !capturedPiece || movingPiece[0] === capturedPiece[0]) return;

    if (capturedPiece[0] === 'b') {
        capturedByMe.push(capturedPiece);
    } else {
        capturedByOpponent.push(capturedPiece);
    }
}

function renderCapturedPieces() {
    renderCapturedTray('me-captured', capturedByMe);
    renderCapturedTray('opponent-captured', capturedByOpponent);
}

function renderCapturedTray(elementId, pieces) {
    const tray = document.getElementById(elementId);
    if (!tray) return;

    tray.innerHTML = '';
    pieces.forEach(piece => {
        const img = document.createElement('img');
        img.src = getPieceSrc(piece);
        img.alt = '';
        img.loading = 'eager';
        img.decoding = 'sync';
        tray.appendChild(img);
    });
}

function renderPieceLegend() {
    const legend = document.getElementById('piece-legend');
    if (!legend) return;

    legend.innerHTML = '';
    ['w', 'b'].forEach(color => {
        PIECE_LABELS.forEach(piece => {
            const item = document.createElement('div');
            item.className = 'piece-legend-item';

            const img = document.createElement('img');
            img.src = getPieceSrc(`${color}${piece.code}`);
            img.alt = '';

            const label = document.createElement('span');
            label.textContent = `- ${color === 'w' ? 'White' : 'Black'} ${piece.title}`;

            item.append(img, label);
            legend.appendChild(item);
        });
    });
}

function paintRenderedClassicSquares(rootSelector = '#myBoard', visualState = null) {
    const light = getVisualSquareStrategy(visualState, 'light');
    const dark = getVisualSquareStrategy(visualState, 'dark');

    document.querySelectorAll(`${rootSelector} .white-1e1d7`).forEach(square => {
        square.style.setProperty('background-color', light.getColor(), 'important');
        square.style.setProperty('background-image', `url("${light.getSrc()}")`, 'important');
        square.style.setProperty('background-size', 'cover', 'important');
        square.style.setProperty('background-position', 'center', 'important');
    });

    document.querySelectorAll(`${rootSelector} .black-3c85d`).forEach(square => {
        square.style.setProperty('background-color', dark.getColor(), 'important');
        square.style.setProperty('background-image', `url("${dark.getSrc()}")`, 'important');
        square.style.setProperty('background-size', 'cover', 'important');
        square.style.setProperty('background-position', 'center', 'important');
    });
}

function getPieceSrc(piece) {
    return getVisualPieceSrc(piece);
}

function getVisualSquareStrategy(visualState, kind) {
    const fallbackId = kind === 'light' ? settings.lightSquareStrategyId : settings.darkSquareStrategyId;
    const square = visualState?.[`${kind}_square`] || visualState?.[`${kind}Square`];
    const strategyId = square?.id
        || visualState?.[`${kind}SquareStrategyId`]
        || visualState?.[`${kind}_square_strategy_id`]
        || fallbackId;
    return getSquareStrategy(strategyId);
}

function getVisualPieceSrc(piece, visualState = null) {
    if (!piece) return '';
    const strategyId = getVisualPieceStrategyId(piece, visualState);
    return getPieceStrategy(strategyId).getSrc(piece);
}

function getVisualPieceStrategyId(piece, visualState = null) {
    const pieceColor = piece[0];
    const pieceType = piece[1];
    const colorKey = pieceColor === 'w' ? 'light' : 'dark';
    const legacyColorKey = pieceColor === 'w' ? 'white' : 'black';
    const pieces = visualState?.pieces || {};
    const scopedStrategy = pieces?.[colorKey]?.[pieceType]
        || pieces?.[legacyColorKey]?.[pieceType]
        || visualState?.[`${colorKey}PieceStrategyByType`]?.[pieceType]
        || visualState?.[`${colorKey}_piece_strategy_by_type`]?.[pieceType];

    if (scopedStrategy) return scopedStrategy;

    const sharedStrategy = typeof pieces?.[colorKey] === 'string'
        ? pieces[colorKey]
        : typeof pieces?.[legacyColorKey] === 'string'
            ? pieces[legacyColorKey]
            : '';

    if (sharedStrategy) {
        return normalizeLoadedPieceStrategyId(sharedStrategy, pieceType, colorKey);
    }

    const strategyByType = piece[0] === 'w'
        ? settings.lightPieceStrategyByType
        : settings.darkPieceStrategyByType;
    return strategyByType[pieceType];
}

function refreshCurrentBoard(resetPosition = false) {
    if (!currentVisualBoardSize || !currentTimeControlMinutes) return;
    renderClassicBoard(currentVisualBoardSize, currentTimeControlMinutes, resetPosition, false, currentGameMode);
}

function renderCustomBoard(host, size, position) {
    const lightSquare = getSquareStrategy(settings.lightSquareStrategyId);
    const darkSquare = getSquareStrategy(settings.darkSquareStrategyId);
    const orientation = currentBoardOrientation();
    const grid = document.createElement('div');
    grid.className = 'custom-board';
    grid.dataset.size = String(size);
    grid.dataset.orientation = orientation;
    grid.style.gridTemplateColumns = `repeat(${size}, minmax(0, 1fr))`;
    grid.style.gridTemplateRows = `repeat(${size}, minmax(0, 1fr))`;

    for (let row = 0; row < size; row += 1) {
        for (let col = 0; col < size; col += 1) {
            const square = document.createElement('div');
            const logicalRow = customLogicalIndex(row, size, orientation);
            const logicalCol = customLogicalIndex(col, size, orientation);
            const key = squareKey(logicalRow, logicalCol);
            const strategy = (logicalRow + logicalCol) % 2 === 0 ? lightSquare : darkSquare;

            square.className = 'custom-square';
            if (selectedCustomSquare === key) {
                square.classList.add('selected');
            }
            square.dataset.square = key;
            square.style.backgroundImage = `url("${strategy.getSrc()}")`;
            square.style.backgroundColor = strategy.getColor();
            square.addEventListener('dragover', event => event.preventDefault());
            square.addEventListener('drop', handleCustomDrop);
            square.addEventListener('click', handleCustomSquareClick);

            appendCustomNotation(square, row, col, logicalRow, logicalCol, size);

            const piece = position[key];
            if (piece) {
                const img = document.createElement('img');
                img.className = 'custom-piece';
                img.src = getPieceSrc(piece);
                img.alt = '';
                img.draggable = false;
                img.dataset.from = key;
                img.addEventListener('dragstart', event => event.preventDefault());
                img.addEventListener('pointerdown', handleCustomPointerDown);
                square.appendChild(img);
            }

            grid.appendChild(square);
        }
    }

    host.appendChild(grid);
}

function customLogicalIndex(index, size, orientation = currentBoardOrientation()) {
    return orientation === 'black' ? size - 1 - index : index;
}

function appendCustomNotation(square, visualRow, visualCol, logicalRow, logicalCol, size) {
    if (visualCol === 0) {
        const rank = document.createElement('span');
        rank.className = 'custom-notation custom-numeric';
        rank.textContent = String(size - logicalRow);
        square.appendChild(rank);
    }

    if (visualRow === size - 1) {
        const file = document.createElement('span');
        file.className = 'custom-notation custom-alpha';
        file.textContent = fileLabel(logicalCol);
        square.appendChild(file);
    }
}

function handleCustomPointerDown(event) {
    if (event.pointerType === 'mouse' && event.button !== 0) return;
    if (!currentCustomPosition) return;

    const sourceImg = event.currentTarget;
    const from = sourceImg.dataset.from;
    const piece = currentCustomPosition[from];
    if (!from || !piece) return;

    if (isCustomBackendGameReady() && !canStartCustomBackendMove(from, piece)) {
        return;
    }

    cancelCustomDrag();
    clearCustomMoveHighlights();
    if (isCustomBackendGameReady()) {
        showCustomMoveHighlights(from, piece);
    }

    customDragState = {
        pointerId: event.pointerId,
        from,
        piece,
        sourceImg,
        sourceSquare: sourceImg.closest('.custom-square'),
        startX: event.clientX,
        startY: event.clientY,
        dragImage: null,
        dragWidth: 0,
        dragHeight: 0,
        targetSquare: null
    };

    sourceImg.addEventListener('pointermove', handleCustomPointerMove);
    sourceImg.addEventListener('pointerup', handleCustomPointerUp);
    sourceImg.addEventListener('pointercancel', handleCustomPointerCancel);
    sourceImg.setPointerCapture?.(event.pointerId);
}

function handleCustomPointerMove(event) {
    const state = customDragState;
    if (!state || state.pointerId !== event.pointerId) return;

    const distance = Math.hypot(event.clientX - state.startX, event.clientY - state.startY);
    if (!state.dragImage && distance < CUSTOM_DRAG_START_THRESHOLD) return;
    if (!state.dragImage) {
        startCustomDragVisual(event);
    }

    event.preventDefault();
    moveCustomDragVisual(event.clientX, event.clientY);
    setCustomDragTarget(getCustomSquareAtPoint(event.clientX, event.clientY));
}

function handleCustomPointerUp(event) {
    const state = customDragState;
    if (!state || state.pointerId !== event.pointerId) return;

    const wasDragging = Boolean(state.dragImage);
    const targetSquare = wasDragging ? getCustomSquareAtPoint(event.clientX, event.clientY) : null;
    const to = targetSquare?.dataset.square || null;
    const from = state.from;

    if (wasDragging) {
        event.preventDefault();
        customDragSuppressClickUntil = Date.now() + CUSTOM_DRAG_CLICK_SUPPRESS_MS;
    }

    cancelCustomDrag();

    if (wasDragging) {
        commitCustomMove(from, to);
    }
}

function handleCustomPointerCancel(event) {
    const state = customDragState;
    if (!state || state.pointerId !== event.pointerId) return;
    if (state.dragImage) {
        customDragSuppressClickUntil = Date.now() + CUSTOM_DRAG_CLICK_SUPPRESS_MS;
    }
    cancelCustomDrag();
    clearCustomMoveHighlights();
}

function startCustomDragVisual(event) {
    const state = customDragState;
    if (!state) return;

    const rect = state.sourceImg.getBoundingClientRect();
    const dragImage = document.createElement('img');
    dragImage.className = 'custom-drag-piece';
    dragImage.src = state.sourceImg.currentSrc || state.sourceImg.src;
    dragImage.alt = '';
    dragImage.style.width = `${rect.width}px`;
    dragImage.style.height = `${rect.height}px`;

    state.dragImage = dragImage;
    state.dragWidth = rect.width;
    state.dragHeight = rect.height;
    selectedCustomSquare = null;

    document.querySelector('#myBoard .custom-square.selected')?.classList.remove('selected');
    document.body.appendChild(dragImage);
    state.sourceImg.classList.add('dragging-source');
    state.sourceSquare?.classList.add('drag-source');
    moveCustomDragVisual(event.clientX, event.clientY);
}

function moveCustomDragVisual(clientX, clientY) {
    const state = customDragState;
    if (!state?.dragImage) return;

    state.dragImage.style.transform = `translate3d(${clientX - state.dragWidth / 2}px, ${clientY - state.dragHeight / 2}px, 0)`;
}

function getCustomSquareAtPoint(clientX, clientY) {
    const element = document.elementFromPoint(clientX, clientY);
    return element?.closest?.('#myBoard .custom-square') || null;
}

function setCustomDragTarget(square) {
    const state = customDragState;
    if (!state || state.targetSquare === square) return;

    state.targetSquare?.classList.remove('drag-target');
    state.targetSquare = null;

    if (square?.dataset.square) {
        if (isCustomBackendGameReady() && !isValidCustomBackendTarget(state.from, square.dataset.square)) {
            return;
        }
        square.classList.add('drag-target');
        state.targetSquare = square;
    }
}

function cancelCustomDrag() {
    const state = customDragState;
    if (!state) return;

    state.targetSquare?.classList.remove('drag-target');
    state.sourceSquare?.classList.remove('drag-source');
    state.sourceImg?.classList.remove('dragging-source');
    state.dragImage?.remove();
    state.sourceImg?.removeEventListener('pointermove', handleCustomPointerMove);
    state.sourceImg?.removeEventListener('pointerup', handleCustomPointerUp);
    state.sourceImg?.removeEventListener('pointercancel', handleCustomPointerCancel);

    if (state.sourceImg?.hasPointerCapture?.(state.pointerId)) {
        state.sourceImg.releasePointerCapture(state.pointerId);
    }

    customDragState = null;
}

function handleCustomDrop(event) {
    event.preventDefault();

    const from = event.dataTransfer.getData('text/plain');
    const to = event.currentTarget.dataset.square;
    commitCustomMove(from, to);
}

function handleCustomSquareClick(event) {
    if (!currentCustomPosition) return;
    if (Date.now() < customDragSuppressClickUntil) return;

    const target = event.currentTarget.dataset.square;
    if (!target) return;

    if (isCustomBackendGameReady()) {
        const targetPiece = currentCustomPosition[target];
        if (selectedCustomSquare && selectedCustomSquare !== target) {
            if (isValidCustomBackendTarget(selectedCustomSquare, target)) {
                commitCustomMove(selectedCustomSquare, target);
                return;
            }
            clearCustomMoveHighlights();
            selectedCustomSquare = null;
            refreshCurrentBoard(false);
        }

        if (targetPiece && canStartCustomBackendMove(target, targetPiece)) {
            selectedCustomSquare = target;
            refreshCurrentBoard(false);
            clearCustomMoveHighlights();
            showCustomMoveHighlights(target, targetPiece);
            return;
        }

        clearCustomMoveHighlights();
        selectedCustomSquare = null;
        refreshCurrentBoard(false);
        return;
    }

    if (selectedCustomSquare && selectedCustomSquare !== target && currentCustomPosition[selectedCustomSquare]) {
        commitCustomMove(selectedCustomSquare, target);
        return;
    }

    selectedCustomSquare = currentCustomPosition[target] ? target : null;
    refreshCurrentBoard(false);
}

function commitCustomMove(from, to) {
    if (!currentCustomPosition || !currentVisualBoardSize) return false;
    if (!from || !to || from === to || !currentCustomPosition[from]) return false;

    if (isCustomBackendGameReady()) {
        return submitCustomBackendMove(from, to);
    }

    const movingPiece = currentCustomPosition[from];
    trackCapture(movingPiece, currentCustomPosition[to]);
    currentCustomPosition[to] = movingPiece;
    delete currentCustomPosition[from];
    selectedCustomSquare = null;
    refreshCurrentBoard(false);
    renderCapturedPieces();
    handleLocalMoveComplete();
    return true;
}

function isCustomBackendGameReady() {
    return activeRemoteGame
        && currentGameState
        && currentPlayerColor
        && boardSizeFromGameState(currentGameState) !== 8;
}

function canStartCustomBackendMove(from, piece) {
    if (pendingClassicMove) {
        setMatchmakingStatus('Waiting for move confirmation.');
        return false;
    }

    if (!window.ChessSocket?.isOpen?.()) {
        setMatchmakingStatus(GAME_CONNECTION_LOST_MESSAGE);
        return false;
    }

    if (currentGameState.status !== 'active') {
        setMatchmakingStatus(currentGameState.status || 'Game is not active.');
        return false;
    }

    if (!isCurrentPlayerTurn()) {
        setMatchmakingStatus('Opponent turn.');
        return false;
    }

    if (!piece || piece[0] !== frontendColorCode(currentPlayerColor)) {
        setMatchmakingStatus('You can move only your pieces.');
        return false;
    }

    const backendFrom = customKeyToBackendSquare(from, currentVisualBoardSize);
    if (validTargetsForSquare(backendFrom).length === 0) {
        setMatchmakingStatus('This piece has no legal moves.');
        return false;
    }

    return true;
}

function submitCustomBackendMove(from, to) {
    const movingPiece = currentCustomPosition[from];
    if (!canStartCustomBackendMove(from, movingPiece)) {
        clearCustomMoveHighlights();
        return false;
    }

    const backendFrom = customKeyToBackendSquare(from, currentVisualBoardSize);
    const backendTo = customKeyToBackendSquare(to, currentVisualBoardSize);
    if (!backendFrom || !backendTo || !validTargetsForSquare(backendFrom).includes(backendTo)) {
        setMatchmakingStatus('Illegal move.');
        clearCustomMoveHighlights();
        selectedCustomSquare = null;
        refreshCurrentBoard(false);
        return false;
    }

    selectedCustomSquare = null;
    clearCustomMoveHighlights();

    if (isPromotionMove(movingPiece, backendTo, currentVisualBoardSize)) {
        showPromotionPicker({ from: backendFrom, to: backendTo, piece: movingPiece, boardSize: currentVisualBoardSize });
        refreshCurrentBoard(false);
        return true;
    }

    submitBackendMove({ from: backendFrom, to: backendTo, snapback: false });
    refreshCurrentBoard(false);
    return true;
}

function isValidCustomBackendTarget(from, to) {
    const backendFrom = customKeyToBackendSquare(from, currentVisualBoardSize);
    const backendTo = customKeyToBackendSquare(to, currentVisualBoardSize);
    return Boolean(backendFrom && backendTo && validTargetsForSquare(backendFrom).includes(backendTo));
}

function customKeyToBackendSquare(key, boardSize) {
    const [rawRow, rawCol] = String(key || '').split('-');
    const row = Number(rawRow);
    const col = Number(rawCol);
    if (!Number.isInteger(row) || !Number.isInteger(col)) return '';
    if (row < 0 || col < 0 || row >= boardSize || col >= boardSize) return '';
    return `${fileLabel(col)}${boardSize - row}`;
}

function showCustomMoveHighlights(source, piece) {
    const sourceSquare = customSquareElement(source);
    sourceSquare?.classList.add('custom-move-source');

    const backendFrom = customKeyToBackendSquare(source, currentVisualBoardSize);
    validTargetsForSquare(backendFrom).forEach(target => {
        const customTarget = backendSquareToCustomKey(target, currentVisualBoardSize);
        const targetSquare = customSquareElement(customTarget);
        if (!targetSquare) return;

        const targetPiece = currentCustomPosition?.[customTarget];
        const targetClass = piece && targetPiece && targetPiece[0] !== piece[0]
            ? 'custom-capture-target'
            : 'custom-move-target';
        targetSquare.classList.add(targetClass);
    });
}

function clearCustomMoveHighlights() {
    document
        .querySelectorAll('#myBoard .custom-move-source, #myBoard .custom-move-target, #myBoard .custom-capture-target')
        .forEach(square => {
            square.classList.remove('custom-move-source', 'custom-move-target', 'custom-capture-target');
        });
}

function customSquareElement(square) {
    if (!square) return null;
    return document.querySelector(`#myBoard .custom-square[data-square="${square}"]`);
}

function buildVisualPosition(size) {
    const rank = buildBackRank(size);
    const position = {};

    rank.forEach((piece, col) => {
        position[squareKey(0, col)] = `b${piece}`;
        position[squareKey(1, col)] = 'bP';
        position[squareKey(size - 2, col)] = 'wP';
        position[squareKey(size - 1, col)] = `w${piece}`;
    });

    return position;
}

function buildBackRank(size) {
    if (size === 8) return ['R', 'N', 'B', 'Q', 'K', 'B', 'N', 'R'];
    if (size === 10) return ['R', 'N', 'N', 'B', 'Q', 'K', 'B', 'N', 'N', 'R'];
    return ['R', 'N', 'B', 'B', 'N', 'Q', 'K', 'N', 'B', 'B', 'N', 'R'];
}

function squareKey(row, col) {
    return `${row}-${col}`;
}

function fileLabel(col) {
    return String.fromCharCode('a'.charCodeAt(0) + col);
}

function renderSettingsGallery() {
    const gallery = document.getElementById('settings-gallery');
    if (!gallery) return;

    gallery.innerHTML = '';
    PIECE_LABELS.forEach(piece => {
        gallery.appendChild(createPieceSection(piece));
    });
    gallery.appendChild(createSquaresSection());
    gallery.appendChild(createBackgroundSection());
    settingsGalleryRendered = true;
}

function createPieceSection(piece) {
    const lightStrategyId = settings.lightPieceStrategyByType[piece.code];
    const darkStrategyId = settings.darkPieceStrategyByType[piece.code];
    const section = createAssetSection({
        title: piece.title,
        iconSrc: getPieceStrategy(lightStrategyId).getSrc(`w${piece.code}`)
    });
    const options = section.querySelector('.asset-options');

    getPieceStrategiesForType(piece.code).forEach(strategy => {
        options.appendChild(createPieceOption(piece.code, strategy, lightStrategyId, darkStrategyId));
    });
    options.appendChild(createPieceUploadOption(piece));

    return section;
}

function createPieceOption(pieceType, strategy, lightStrategyId, darkStrategyId) {
    const option = document.createElement('div');
    const isActive = strategy.id === lightStrategyId || strategy.id === darkStrategyId;
    option.className = `asset-option piece-option ${isActive ? 'active' : ''}`;

    const preview = document.createElement('img');
    preview.className = 'asset-piece-preview';
    configurePreviewImage(preview, strategy.getSrc(`w${pieceType}`));

    const name = document.createElement('span');
    name.className = 'asset-option-name';
    name.textContent = strategy.name;

    const controls = document.createElement('span');
    controls.className = 'square-role-controls';
    controls.append(
        createAssetRoleControl('Light', strategy.id, lightStrategyId, () => selectPieceStrategy('light', pieceType, strategy.id)),
        createAssetRoleControl('Dark', strategy.id, darkStrategyId, () => selectPieceStrategy('dark', pieceType, strategy.id))
    );

    option.append(preview, name, controls);
    return option;
}

function createPieceUploadOption(piece) {
    const wrapper = document.createElement('div');
    wrapper.className = 'asset-option upload-option';

    const plus = document.createElement('span');
    plus.className = 'upload-plus';
    plus.textContent = '+';

    const nameInput = document.createElement('input');
    nameInput.className = 'upload-name-input';
    nameInput.type = 'text';
    nameInput.placeholder = `${piece.title} variant name`;

    const choices = document.createElement('span');
    choices.className = 'piece-upload-actions';
    choices.append(createPieceUploadButton(piece, nameInput));

    wrapper.append(plus, nameInput, choices);
    return wrapper;
}

function createPieceUploadButton(piece, nameInput) {
    const fileInput = document.createElement('input');
    fileInput.type = 'file';
    fileInput.accept = '.png,.jpg,.jpeg,.svg,.gif,image/png,image/jpeg,image/svg+xml,image/gif';
    fileInput.addEventListener('change', async event => {
        const [file] = Array.from(event.target.files || []);
        if (!file) return;

        try {
            const variant = await createUserPieceVariant(piece.code, file, nameInput.value.trim());
            userStyles.pieceVariants.push(variant);
            persistUserStyles();
            renderSettingsGallery();
            showSettingsMessage(`Piece variant saved: ${variant.name}`);
        } catch (error) {
            showSettingsMessage(error.message);
        } finally {
            event.target.value = '';
        }
    });

    const label = document.createElement('label');
    label.className = 'inline-upload-btn piece-upload-btn';
    label.append(document.createTextNode('Add Piece'), fileInput);
    return label;
}

function createSquaresSection() {
    const lightStrategy = getSquareStrategy(settings.lightSquareStrategyId);
    const darkStrategy = getSquareStrategy(settings.darkSquareStrategyId);
    const section = createAssetSection({
        title: 'Board Squares',
        squareSrc: lightStrategy.getSrc()
    });
    const options = section.querySelector('.asset-options');
    options.classList.add('square-options');

    getAllSquareStrategies().forEach(strategy => {
        options.appendChild(createSquareOption(strategy, lightStrategy.id, darkStrategy.id));
    });
    options.appendChild(createSquareUploadOption());

    return section;
}

function createBackgroundSection() {
    const section = createAssetSection({
        title: 'Background',
        backgroundPreviewClass: getBackgroundStrategy(settings.backgroundStrategyId).getPreviewClass()
    });
    const options = section.querySelector('.asset-options');
    options.classList.add('background-options');

    backgroundStrategies.forEach(strategy => {
        options.appendChild(createBackgroundOption(strategy));
    });
    options.appendChild(createFallingPiecesOption());

    return section;
}

function createBackgroundOption(strategy) {
    const option = document.createElement('div');
    const isActive = strategy.id === settings.backgroundStrategyId;
    option.className = `asset-option background-option ${isActive ? 'active' : ''}`;

    const preview = document.createElement('span');
    preview.className = `background-preview ${strategy.getPreviewClass()}`;

    const name = document.createElement('span');
    name.className = 'asset-option-name';
    name.textContent = strategy.name;

    const controls = document.createElement('span');
    controls.className = 'square-role-controls single-role-controls';
    controls.append(createAssetRoleControl('Use', strategy.id, settings.backgroundStrategyId, () => selectBackgroundStrategy(strategy.id)));

    option.append(preview, name, controls);
    return option;
}

function createFallingPiecesOption() {
    const option = document.createElement('div');
    option.className = `asset-option falling-pieces-option ${settings.fallingPiecesEnabled ? 'active' : ''}`;

    const preview = document.createElement('span');
    preview.className = 'falling-pieces-preview';

    ['wP', 'bN', 'wK'].forEach(piece => {
        const img = document.createElement('img');
        configurePreviewImage(img, getPieceSrc(piece));
        preview.appendChild(img);
    });

    const name = document.createElement('span');
    name.className = 'asset-option-name';
    name.textContent = 'Falling Pieces';

    const label = document.createElement('label');
    label.className = 'square-role-control';
    const checkbox = document.createElement('input');
    checkbox.type = 'checkbox';
    checkbox.checked = settings.fallingPiecesEnabled;
    checkbox.addEventListener('change', event => {
        selectFallingPiecesEnabled(event.currentTarget.checked);
    });
    label.append(checkbox, document.createTextNode('Enabled'));

    const controls = document.createElement('span');
    controls.className = 'square-role-controls single-role-controls';
    controls.append(label);

    option.append(preview, name, controls);
    return option;
}

function createSquareOption(strategy, lightId, darkId) {
    const option = document.createElement('div');
    option.className = 'asset-option square-option';

    const swatch = document.createElement('span');
    swatch.className = 'square-swatch';
    swatch.style.backgroundImage = `url("${strategy.getSrc()}")`;
    swatch.style.backgroundColor = strategy.getColor();

    const name = document.createElement('span');
    name.className = 'asset-option-name';
    name.textContent = strategy.name;

    const controls = document.createElement('span');
    controls.className = 'square-role-controls';
    controls.append(
        createAssetRoleControl('Light', strategy.id, lightId, () => selectSquareStrategy('light', strategy.id)),
        createAssetRoleControl('Dark', strategy.id, darkId, () => selectSquareStrategy('dark', strategy.id))
    );

    option.append(swatch, name, controls);
    return option;
}

function createAssetRoleControl(labelText, strategyId, selectedId, onSelect) {
    const label = document.createElement('label');
    label.className = 'square-role-control';

    const checkbox = document.createElement('input');
    checkbox.type = 'checkbox';
    checkbox.checked = strategyId === selectedId;
    checkbox.addEventListener('change', event => {
        if (event.currentTarget.checked) {
            onSelect();
            return;
        }
        event.currentTarget.checked = true;
    });

    label.append(checkbox, document.createTextNode(labelText));
    return label;
}

function createSquareUploadOption() {
    const wrapper = document.createElement('div');
    wrapper.className = 'asset-option upload-option square-upload-option';

    const plus = document.createElement('span');
    plus.className = 'upload-plus';
    plus.textContent = '+';

    const nameInput = document.createElement('input');
    nameInput.className = 'upload-name-input';
    nameInput.type = 'text';
    nameInput.placeholder = 'Square variant name';

    const fileInput = document.createElement('input');
    fileInput.type = 'file';
    fileInput.accept = '.png,.jpg,.jpeg,.svg,.gif,image/png,image/jpeg,image/svg+xml,image/gif';
    fileInput.addEventListener('change', async event => {
        const [file] = Array.from(event.target.files || []);
        if (!file) return;

        try {
            const variant = await createUserSquareVariant(file, nameInput.value.trim());
            userStyles.squareVariants.push(variant);
            persistUserStyles();
            selectSquareStrategy('light', variant.id);
            showSettingsMessage(`Square variant saved: ${variant.name}`);
        } catch (error) {
            showSettingsMessage(error.message);
        } finally {
            event.target.value = '';
        }
    });

    const label = document.createElement('label');
    label.className = 'inline-upload-btn';
    label.append(plus, document.createTextNode('Upload Square'), fileInput);

    wrapper.append(label, nameInput);
    return wrapper;
}

function createAssetSection({ title, iconSrc, squareSrc, backgroundPreviewClass }) {
    const section = document.createElement('section');
    section.className = 'asset-section';

    const header = document.createElement('div');
    header.className = 'asset-header';

    if (iconSrc) {
        const icon = document.createElement('img');
        icon.className = 'asset-icon';
        configurePreviewImage(icon, iconSrc);
        header.appendChild(icon);
    } else if (backgroundPreviewClass) {
        const icon = document.createElement('span');
        icon.className = `asset-icon background-preview ${backgroundPreviewClass}`;
        header.appendChild(icon);
    } else {
        const icon = document.createElement('span');
        icon.className = 'asset-icon square-swatch';
        icon.style.backgroundImage = `url("${squareSrc}")`;
        header.appendChild(icon);
    }

    const heading = document.createElement('h3');
    heading.className = 'asset-title';
    heading.textContent = title;

    const options = document.createElement('div');
    options.className = 'asset-options';

    header.appendChild(heading);
    section.append(header, options);
    return section;
}

function configurePreviewImage(image, src) {
    image.alt = '';
    image.loading = 'lazy';
    image.decoding = 'async';
    image.fetchPriority = 'low';
    image.src = src;
}

function selectPieceStrategy(kind, pieceType, strategyId) {
    if (kind === 'light') {
        settings.lightPieceStrategyByType[pieceType] = strategyId;
        if (settings.darkPieceStrategyByType[pieceType] === strategyId) {
            settings.darkPieceStrategyByType[pieceType] = fallbackPieceStrategyId(pieceType, 'dark', strategyId);
        }
    } else {
        settings.darkPieceStrategyByType[pieceType] = strategyId;
        if (settings.lightPieceStrategyByType[pieceType] === strategyId) {
            settings.lightPieceStrategyByType[pieceType] = fallbackPieceStrategyId(pieceType, 'light', strategyId);
        }
    }

    saveCurrentSettings();
    if (settingsGalleryRendered) {
        renderSettingsGallery();
    }
    refreshCurrentBoard(false);
    renderCapturedPieces();
    renderPieceLegend();
}

function fallbackPieceStrategyId(pieceType, kind, avoidStrategyId) {
    const defaultId = defaultPieceStrategyId(pieceType, kind);
    if (defaultId !== avoidStrategyId) return defaultId;

    const fallback = getPieceStrategiesForType(pieceType).find(strategy => strategy.id !== avoidStrategyId);
    return fallback?.id || builtInPieceStrategies[0].id;
}

function defaultPieceStrategyId(pieceType, kind) {
    return `${builtInPieceStrategies[0].id}-${pieceType}-${kind}`;
}

function selectSquareStrategy(kind, strategyId) {
    if (kind === 'light') {
        settings.lightSquareStrategyId = strategyId;
    } else {
        settings.darkSquareStrategyId = strategyId;
    }

    saveCurrentSettings();
    applySelectedBoardSquares();
    if (settingsGalleryRendered) {
        renderSettingsGallery();
    }
    refreshCurrentBoard(false);
}

function selectBackgroundStrategy(strategyId) {
    settings.backgroundStrategyId = strategyId;
    saveCurrentSettings();
    applySelectedBackground();
    if (settingsGalleryRendered) {
        renderSettingsGallery();
    }
}

function selectFallingPiecesEnabled(enabled) {
    settings.fallingPiecesEnabled = enabled;
    saveCurrentSettings();
    applyFallingPiecesPreference();
    if (settingsGalleryRendered) {
        renderSettingsGallery();
    }
}

function applySelectedBackground() {
    getBackgroundStrategy(settings.backgroundStrategyId).apply();
}

function applySelectedBoardSquares() {
    const light = getSquareStrategy(settings.lightSquareStrategyId);
    const dark = getSquareStrategy(settings.darkSquareStrategyId);
    const root = document.documentElement;

    root.style.setProperty('--board-light-color', light.getColor());
    root.style.setProperty('--board-dark-color', dark.getColor());
    root.style.setProperty('--board-light-image', `url("${light.getSrc()}")`);
    root.style.setProperty('--board-dark-image', `url("${dark.getSrc()}")`);
    paintRenderedClassicSquares();
}

async function createUserPieceVariant(pieceType, file, requestedName) {
    const src = await readFileAsDataUrl(file);
    const fileName = file.name.replace(/\.[^.]+$/, '').replace(/[-_]+/g, ' ').trim();

    return {
        id: createUserId(`piece-${pieceType.toLowerCase()}`),
        name: requestedName || fileName || `Custom ${PIECE_NAMES[pieceType]}`,
        pieceType,
        src,
        whiteSrc: src,
        blackSrc: src
    };
}

async function createUserSquareVariant(file, requestedName) {
    return {
        id: createUserId('square'),
        name: requestedName || 'Custom Square',
        src: await readFileAsDataUrl(file),
        color: '#f0d9b5'
    };
}

function readFileAsDataUrl(file) {
    return new Promise((resolve, reject) => {
        const reader = new FileReader();
        reader.onload = () => resolve(reader.result);
        reader.onerror = () => reject(new Error(`Cannot read ${file.name}`));
        reader.readAsDataURL(file);
    });
}

async function readAccountAvatarAsDataUrl(file) {
    if (!file.type.startsWith('image/')) {
        throw new Error('Profile image must be an image file.');
    }

    const source = await readFileAsDataUrl(file);
    const image = await loadImage(source);
    const scale = Math.min(1, ACCOUNT_AVATAR_SIZE / Math.max(image.width, image.height));
    const width = Math.max(1, Math.round(image.width * scale));
    const height = Math.max(1, Math.round(image.height * scale));
    const canvas = document.createElement('canvas');
    canvas.width = width;
    canvas.height = height;
    const context = canvas.getContext('2d');
    context.imageSmoothingEnabled = true;
    context.imageSmoothingQuality = 'high';
    context.drawImage(image, 0, 0, width, height);
    return canvas.toDataURL('image/webp', 0.82);
}

function loadImage(src) {
    return new Promise((resolve, reject) => {
        const image = new Image();
        image.onload = () => resolve(image);
        image.onerror = () => reject(new Error('Cannot load profile image.'));
        image.src = src;
    });
}

function getAllPieceStrategies() {
    return [
        ...getBuiltInSinglePieceStrategies(),
        ...userStyles.pieceVariants.map(variant => new UploadedPieceVariantStrategy(variant))
    ];
}

function getPieceStrategiesForType(pieceType) {
    return getAllPieceStrategies().filter(strategy => !strategy.pieceType || strategy.pieceType === pieceType);
}

function getPieceStrategy(strategyId) {
    return getAllPieceStrategies().find(strategy => strategy.id === strategyId)
        || builtInPieceStrategies.find(strategy => strategy.id === strategyId)
        || getBuiltInSinglePieceStrategies()[0];
}

function getBuiltInSinglePieceStrategies() {
    return builtInPieceStrategies.flatMap(strategy => (
        PIECE_TYPES.flatMap(pieceType => [
            new SinglePieceImageStrategy({ baseStrategy: strategy, pieceType, sourceColor: 'w' }),
            new SinglePieceImageStrategy({ baseStrategy: strategy, pieceType, sourceColor: 'b' })
        ])
    ));
}

function getAllSquareStrategies() {
    return [
        ...builtInSquareStrategies,
        ...userStyles.squareVariants.map(variant => new UploadedSquareStrategy(variant))
    ];
}

function getSquareStrategy(strategyId) {
    return getAllSquareStrategies().find(strategy => strategy.id === strategyId) || builtInSquareStrategies[0];
}

function getBackgroundStrategy(strategyId) {
    return backgroundStrategies.find(strategy => strategy.id === strategyId) || backgroundStrategies[0];
}

function loadUserStyles() {
    try {
        const parsed = JSON.parse(localStorage.getItem(USER_STYLES_KEY));
        const normalized = normalizeUserStyles(parsed);
        if (parsed?.version !== USER_STYLES_VERSION) {
            localStorage.setItem(USER_STYLES_KEY, JSON.stringify(normalized));
        }
        return normalized;
    } catch {
        return normalizeUserStyles();
    }
}

function normalizeUserStyles(parsed = {}) {
    const shouldResetPieceVariants = parsed.version !== USER_STYLES_VERSION;

    return {
        version: USER_STYLES_VERSION,
        pieceVariants: shouldResetPieceVariants
            ? []
            : normalizeSinglePieceVariants(Array.isArray(parsed.pieceVariants) ? parsed.pieceVariants : migrateOldPieceVariants(parsed.pieces)),
        squareVariants: Array.isArray(parsed.squareVariants) ? parsed.squareVariants : migrateOldSquareVariants(parsed.boards)
    };
}

function normalizeSinglePieceVariants(variants) {
    return variants.map(variant => {
        const src = variant.src || variant.whiteSrc || variant.blackSrc;
        const role = variant.role || (/(^|\s|[-_])dark($|\s|[-_])|black|(^|[-_])b[-_]?/i.test(variant.name || '') ? 'dark' : 'light');
        return {
            ...variant,
            role,
            src,
            whiteSrc: src,
            blackSrc: src
        };
    }).filter(variant => variant.src);
}

function migrateOldPieceVariants(oldPieces) {
    if (!Array.isArray(oldPieces)) return [];

    return oldPieces.flatMap(style => {
        if (!style?.pieces) return [];
        return PIECE_TYPES.map(pieceType => ({
            id: `${style.id}-${pieceType}`,
            name: `${style.name} ${PIECE_NAMES[pieceType]}`,
            pieceType,
            whiteSrc: style.pieces[`w${pieceType}`],
            blackSrc: style.pieces[`b${pieceType}`]
        })).filter(variant => variant.whiteSrc && variant.blackSrc);
    });
}

function migrateOldSquareVariants(oldBoards) {
    if (!Array.isArray(oldBoards)) return [];

    return oldBoards.flatMap(style => {
        const variants = [];
        if (style.light) {
            variants.push({
                id: `${style.id}-light`,
                name: `${style.name} Light`,
                src: style.light,
                color: style.lightColor
            });
        }
        if (style.dark) {
            variants.push({
                id: `${style.id}-dark`,
                name: `${style.name} Dark`,
                src: style.dark,
                color: style.darkColor
            });
        }
        return variants;
    });
}

function persistUserStyles() {
    try {
        localStorage.setItem(USER_STYLES_KEY, JSON.stringify(userStyles));
    } catch {
        throw new Error('Browser storage is full. Use smaller images.');
    }
}

function loadCurrentSettings() {
    const defaults = defaultSettings();

    try {
        const parsed = JSON.parse(localStorage.getItem(CURRENT_SETTINGS_KEY));
        if (!parsed) return defaults;

        const isCurrentSettingsVersion = parsed.version === CURRENT_SETTINGS_VERSION;
        const migratedPieceStyle = parsed.pieceStyleId || 'classic';
        const lightPieceStrategyByType = { ...defaults.lightPieceStrategyByType };
        const darkPieceStrategyByType = { ...defaults.darkPieceStrategyByType };
        PIECE_TYPES.forEach(type => {
            const legacyStrategyId = parsed.pieceStrategyByType?.[type] || parsed.pieceStyleByType?.[type] || migratedPieceStyle;
            lightPieceStrategyByType[type] = normalizeLoadedPieceStrategyId(parsed.lightPieceStrategyByType?.[type] || legacyStrategyId, type, 'light');
            darkPieceStrategyByType[type] = normalizeLoadedPieceStrategyId(parsed.darkPieceStrategyByType?.[type] || legacyStrategyId, type, 'dark');
        });

        return {
            version: CURRENT_SETTINGS_VERSION,
            lightPieceStrategyByType,
            darkPieceStrategyByType,
            lightSquareStrategyId: parsed.lightSquareStrategyId || parsed.lightSquareStyleId || parsed.boardStyleId || defaults.lightSquareStrategyId,
            darkSquareStrategyId: parsed.darkSquareStrategyId || parsed.darkSquareStyleId || parsed.boardStyleId || defaults.darkSquareStrategyId,
            backgroundStrategyId: parsed.backgroundStrategyId || defaults.backgroundStrategyId,
            fallingPiecesEnabled: isCurrentSettingsVersion && typeof parsed.fallingPiecesEnabled === 'boolean'
                ? parsed.fallingPiecesEnabled
                : defaults.fallingPiecesEnabled
        };
    } catch {
        return defaults;
    }
}

function normalizeLoadedPieceStrategyId(strategyId, pieceType, kind) {
    if (builtInPieceStrategies.some(strategy => strategy.id === strategyId)) {
        return `${strategyId}-${pieceType}-${kind}`;
    }
    return strategyId || defaultPieceStrategyId(pieceType, kind);
}

function defaultSettings() {
    return {
        version: CURRENT_SETTINGS_VERSION,
        lightPieceStrategyByType: PIECE_TYPES.reduce((result, type) => {
            result[type] = defaultPieceStrategyId(type, 'light');
            return result;
        }, {}),
        darkPieceStrategyByType: PIECE_TYPES.reduce((result, type) => {
            result[type] = defaultPieceStrategyId(type, 'dark');
            return result;
        }, {}),
        lightSquareStrategyId: 'yellow-square',
        darkSquareStrategyId: 'classic-green-square',
        backgroundStrategyId: 'cozy-board',
        fallingPiecesEnabled: false
    };
}

function normalizeSettings() {
    const pieceIds = getAllPieceStrategies().map(strategy => strategy.id);
    const squareIds = getAllSquareStrategies().map(strategy => strategy.id);
    const backgroundIds = backgroundStrategies.map(strategy => strategy.id);
    settings.version = CURRENT_SETTINGS_VERSION;
    settings.lightPieceStrategyByType ||= {};
    settings.darkPieceStrategyByType ||= {};

    PIECE_TYPES.forEach(type => {
        settings.lightPieceStrategyByType[type] = normalizeLoadedPieceStrategyId(settings.lightPieceStrategyByType[type], type, 'light');
        settings.darkPieceStrategyByType[type] = normalizeLoadedPieceStrategyId(settings.darkPieceStrategyByType[type], type, 'dark');

        if (!pieceIds.includes(settings.lightPieceStrategyByType[type])) {
            settings.lightPieceStrategyByType[type] = defaultPieceStrategyId(type, 'light');
        }
        if (!pieceIds.includes(settings.darkPieceStrategyByType[type])) {
            settings.darkPieceStrategyByType[type] = defaultPieceStrategyId(type, 'dark');
        }
        if (settings.lightPieceStrategyByType[type] === settings.darkPieceStrategyByType[type]) {
            settings.darkPieceStrategyByType[type] = fallbackPieceStrategyId(type, 'dark', settings.lightPieceStrategyByType[type]);
        }
    });

    if (!squareIds.includes(settings.lightSquareStrategyId)) {
        settings.lightSquareStrategyId = 'yellow-square';
    }

    if (!squareIds.includes(settings.darkSquareStrategyId)) {
        settings.darkSquareStrategyId = 'classic-green-square';
    }

    if (!backgroundIds.includes(settings.backgroundStrategyId)) {
        settings.backgroundStrategyId = 'cozy-board';
    }

    if (typeof settings.fallingPiecesEnabled !== 'boolean') {
        settings.fallingPiecesEnabled = false;
    }

    saveCurrentSettings();
}

function saveCurrentSettings() {
    localStorage.setItem(CURRENT_SETTINGS_KEY, JSON.stringify(settings));
}

function createUserId(prefix) {
    return `user-${prefix}-${Date.now()}`;
}

function showSettingsMessage(message) {
    const messageEl = document.getElementById('settings-message');
    if (messageEl) {
        messageEl.textContent = message;
    }
}
