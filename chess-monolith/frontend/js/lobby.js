const ASSET_ROOT = 'images';
const PIECES_ROOT = `${ASSET_ROOT}/сhess_pieces`;
const USER_STYLES_KEY = 'chessemag_user_styles';
const USER_STYLES_VERSION = 2;
const CURRENT_SETTINGS_KEY = 'chessemag_current_settings';
const LEGACY_ACCOUNT_PROFILE_KEY = 'chessemag_account_profile';
const ACCOUNT_AVATAR_SIZE = 256;
const TIMER_ROOT = `${ASSET_ROOT}/timer`;
const TIMER_DIGIT_ROOT = `${TIMER_ROOT}/digits`;
const CUSTOM_DRAG_START_THRESHOLD = 5;
const CUSTOM_DRAG_CLICK_SUPPRESS_MS = 250;

const PIECE_TYPES = ['P', 'R', 'N', 'B', 'Q', 'K'];
const PIECE_NAMES = {
    K: 'King',
    Q: 'Queen',
    R: 'Rook',
    B: 'Bishop',
    N: 'Horse',
    P: 'Pawn'
};
const PIECE_LABELS = [
    { code: 'P', title: 'Pawn' },
    { code: 'R', title: 'Rook' },
    { code: 'N', title: 'Knight' },
    { code: 'B', title: 'Bishop' },
    { code: 'Q', title: 'Queen' },
    { code: 'K', title: 'King' }
];

class PieceAssetStrategy {
    constructor({ id, name, pieceType = null }) {
        this.id = id;
        this.name = name;
        this.pieceType = pieceType;
    }

    getSrc() {
        throw new Error('PieceAssetStrategy#getSrc must be implemented.');
    }
}

class BuiltInPieceStyleStrategy extends PieceAssetStrategy {
    constructor({ id, name, src }) {
        super({ id, name });
        this.src = src;
    }

    getSrc(piece) {
        return this.src(piece);
    }
}

class UploadedPieceVariantStrategy extends PieceAssetStrategy {
    constructor({ id, name, pieceType, role = 'light', whiteSrc, blackSrc, src }) {
        super({ id, name, pieceType });
        this.role = role;
        this.whiteSrc = whiteSrc || src || blackSrc;
        this.blackSrc = blackSrc || src || whiteSrc;
    }

    getSrc(piece) {
        return piece[0] === 'w' ? this.whiteSrc : this.blackSrc;
    }
}

class SinglePieceImageStrategy extends PieceAssetStrategy {
    constructor({ baseStrategy, pieceType, sourceColor }) {
        super({
            id: `${baseStrategy.id}-${pieceType}-${sourceColor === 'w' ? 'light' : 'dark'}`,
            name: `${baseStrategy.name} ${sourceColor === 'w' ? 'Light' : 'Dark'}`,
            pieceType
        });
        this.baseStrategy = baseStrategy;
        this.sourceColor = sourceColor;
    }

    getSrc() {
        return this.baseStrategy.getSrc(`${this.sourceColor}${this.pieceType}`);
    }
}

class SquareAssetStrategy {
    constructor({ id, name, color = '#f0d9b5' }) {
        this.id = id;
        this.name = name;
        this.color = color;
    }

    getSrc() {
        throw new Error('SquareAssetStrategy#getSrc must be implemented.');
    }

    getColor() {
        return this.color;
    }
}

class BuiltInSquareStrategy extends SquareAssetStrategy {
    constructor({ id, name, src, color }) {
        super({ id, name, color });
        this.src = src;
    }

    getSrc() {
        return this.src;
    }
}

class UploadedSquareStrategy extends SquareAssetStrategy {
    constructor({ id, name, src, color }) {
        super({ id, name, color });
        this.src = src;
    }

    getSrc() {
        return this.src;
    }
}

class BackgroundStrategy {
    constructor({ id, name, previewClass }) {
        this.id = id;
        this.name = name;
        this.previewClass = previewClass;
    }

    getPreviewClass() {
        return this.previewClass;
    }

    apply() {
        document.body.dataset.background = this.id;
    }
}

class MatchmakingStrategy {
    findMatch() {
        throw new Error('MatchmakingStrategy#findMatch must be implemented.');
    }

    cancel() {
        throw new Error('MatchmakingStrategy#cancel must be implemented.');
    }
}

class WebSocketMatchmakingStrategy extends MatchmakingStrategy {
    async findMatch({ mode, boardSize, timeControlMinutes }) {
        const token = window.ChessApi?.getToken?.();
        if (!token) {
            throw new Error('Log in before searching for a match.');
        }

        if (!window.ChessSocket) {
            throw new Error('WebSocket client is not loaded.');
        }

        await ChessSocket.connect(token);
        ChessSocket.joinQueue({
            mode,
            boardSize,
            isRanked: false,
            timeControlMinutes
        });

        return {
            mode,
            boardSize,
            timeControlMinutes,
            status: 'waiting',
            message: `Searching for ${modeLabel(mode, boardSize)} · ${timeControlMinutes} min.`
        };
    }

    cancel() {
        window.ChessSocket?.cancelQueue?.();
        return Promise.resolve({ status: 'cancelled' });
    }
}

const matchmakingClient = new WebSocketMatchmakingStrategy();

const builtInPieceStrategies = [
    new BuiltInPieceStyleStrategy({
        id: 'classic',
        name: 'Classic',
        src(piece) {
            const color = piece[0] === 'w' ? 'White' : 'Black';
            return `${PIECES_ROOT}/classic_chess/${color}_${PIECE_NAMES[piece[1]]}.png`;
        }
    }),
    new BuiltInPieceStyleStrategy({
        id: 'cheese',
        name: 'Cheese',
        src(piece) {
            const color = piece[0] === 'w' ? 'White' : 'Black';
            return `${PIECES_ROOT}/cheese_chess_alpha/${color}_${PIECE_NAMES[piece[1]]}_cheese.png`;
        }
    }),
    new BuiltInPieceStyleStrategy({
        id: 'cats',
        name: 'Cats',
        src(piece) {
            const color = piece[0] === 'w' ? 'White' : 'Black';
            const name = piece[1] === 'N' ? 'Knight' : PIECE_NAMES[piece[1]];
            return `${PIECES_ROOT}/cats_gen/${color}_Cat_${name}.png`;
        }
    }),
    new BuiltInPieceStyleStrategy({
        id: 'cheese-mice-pixel',
        name: 'Cheese Mice Pixel',
        src(piece) {
            const color = piece[0] === 'w' ? 'White' : 'Black';
            return `${PIECES_ROOT}/cheese_mice_pixel/${color}_${PIECE_NAMES[piece[1]]}_cheese_mouse.png`;
        }
    }),
    new BuiltInPieceStyleStrategy({
        id: 'cheese-mice-svg',
        name: 'Cheese Mice SVG',
        src(piece) {
            const color = piece[0] === 'w' ? 'White' : 'Black';
            return `${PIECES_ROOT}/cheese_mice_svg/${color}_${PIECE_NAMES[piece[1]]}_cheese_mouse.svg`;
        }
    })
];

const builtInSquareStrategies = [
    new BuiltInSquareStrategy({
        id: 'yellow-square',
        name: 'Yellow Square',
        src: `${ASSET_ROOT}/squares/Yellow_Square.png`,
        color: '#f2cf76'
    }),
    new BuiltInSquareStrategy({
        id: 'classic-green-square',
        name: 'Classic Green',
        src: `${ASSET_ROOT}/squares/Classic_Green.png`,
        color: '#73b765'
    }),
    new BuiltInSquareStrategy({
        id: 'green-square',
        name: 'Green Square',
        src: `${ASSET_ROOT}/squares/Green_Square.png`,
        color: '#9bcfbd'
    }),
    new BuiltInSquareStrategy({
        id: 'default-red-square',
        name: 'Default Red',
        src: `${ASSET_ROOT}/squares/Default_Red.png`,
        color: '#e45b45'
    }),
    new BuiltInSquareStrategy({
        id: 'cheese-light-pixel',
        name: 'Cheese Light Pixel',
        src: `${ASSET_ROOT}/squares/cheese_board/Cheese_Light_pixel.png`,
        color: '#eee2c6'
    }),
    new BuiltInSquareStrategy({
        id: 'cheese-dark-pixel',
        name: 'Cheese Dark Pixel',
        src: `${ASSET_ROOT}/squares/cheese_board/Cheese_Dark_pixel.png`,
        color: '#624b32'
    }),
    new BuiltInSquareStrategy({
        id: 'cheese-light-svg',
        name: 'Cheese Light SVG',
        src: `${ASSET_ROOT}/squares/cheese_board/Cheese_Light.svg`,
        color: '#f3ead2'
    }),
    new BuiltInSquareStrategy({
        id: 'cheese-dark-svg',
        name: 'Cheese Dark SVG',
        src: `${ASSET_ROOT}/squares/cheese_board/Cheese_Dark.svg`,
        color: '#6a5138'
    })
];

const AMBIENT_SQUARE_IDS = ['yellow-square', 'classic-green-square', 'green-square', 'default-red-square'];
let ambientResizeTimeoutId = null;
let viewportResizeTimeoutId = null;
let ambientPieceIntervalId = null;
let ambientPieceTimeoutIds = [];
let ambientGridSignature = '';
const preloadedAssetImages = new Map();

const backgroundStrategies = [
    new BackgroundStrategy({
        id: 'cozy-board',
        name: 'Cozy Board',
        previewClass: 'background-preview-cozy'
    }),
    new BackgroundStrategy({
        id: 'dark-room',
        name: 'Dark Room',
        previewClass: 'background-preview-dark'
    })
];

const emojiChatItems = [
    { id: 'shark', name: 'Shark grin', src: `${ASSET_ROOT}/smiles/shark_grin.png` },
    { id: 'bite', name: 'Lip bite', src: `${ASSET_ROOT}/smiles/lip_bite.png` },
    { id: 'clown', name: 'Clown', src: `${ASSET_ROOT}/smiles/clown.png` },
    { id: 'think', name: 'Thinking', src: `${ASSET_ROOT}/smiles/thinking.png` },
    { id: 'cry', name: 'Crying', src: `${ASSET_ROOT}/smiles/crying.png` },
    { id: 'thumb', name: 'Thumbs up', src: `${ASSET_ROOT}/smiles/thumbs_up.png` },
    { id: 'cheese', name: 'Cheese grin', src: `${ASSET_ROOT}/smiles/cheese_grin.png` },
    { id: 'crown', name: 'Crowned', src: `${ASSET_ROOT}/smiles/crowned.png` },
    { id: 'dizzy', name: 'Dizzy', src: `${ASSET_ROOT}/smiles/dizzy.png` },
    { id: 'fire', name: 'On fire', src: `${ASSET_ROOT}/smiles/on_fire.png` },
    { id: 'sus', name: 'Suspicious', src: `${ASSET_ROOT}/smiles/suspicious.png` },
    { id: 'sleep', name: 'Sleepy', src: `${ASSET_ROOT}/smiles/sleepy.png` },
    { id: 'party', name: 'Party', src: `${ASSET_ROOT}/smiles/party.png` },
    { id: 'cool', name: 'Cool', src: `${ASSET_ROOT}/smiles/cool.png` },
    { id: 'rocket', name: 'Rocket mood', src: `${ASSET_ROOT}/smiles/rocket_mood.png` }
];

const historyRecords = [];
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
let selectedModernBoardSize = null;
let selectedModernTimeMinutes = null;
let capturedByMe = [];
let capturedByOpponent = [];
let activeMatchRequest = null;
let queuedForMatch = false;
let activeRemoteGame = false;
let currentGameState = null;
let currentPlayerColor = null;
let currentGameId = null;
let currentValidMoves = {};
let pendingClassicMove = null;
let classicSnapbackInProgress = false;
let queuedClassicPositionUpdate = null;
let historySortDirection = 'desc';
let historyFilters = new Set();
let timerState = null;
let timerIntervalId = null;
let matchNotFoundTimeoutId = null;
let settingsGalleryRendered = false;
let emojiChatRendered = false;
let emojiMessages = [];
let userStyles = loadUserStyles();
let settings = loadCurrentSettings();
let accountProfile = loadAccountProfile();
let accountEditing = false;

document.addEventListener('DOMContentLoaded', () => {
    clearLegacyAccountProfile();
    clearLocalHistoryRecords();
    normalizeSettings();
    bindClassicSetupControls();
    bindModernSetupControls();
    bindHistoryControls();
    bindSocketEvents();
    bindAccountForm();
    renderAccountProfile();
    refreshAccountFromBackend();
    setAccountEntryVisibility('page-menu');
    setPageScrollMode('page-menu');
    applySelectedBackground();
    applySelectedBoardSquares();
    initAmbientBackground();
    warmSettingsAssetCache();
    renderHistoryList();
});

window.addEventListener('resize', () => {
    markViewportResizing();
    if (!board) return;
    board.resize();
    paintRenderedClassicSquares();
});

function navigateTo(pageId) {
    const leavingClassic = document.getElementById('page-classic')?.classList.contains('active') && pageId !== 'page-classic';
    if (leavingClassic) {
        resetClassicEntry();
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
        renderHistoryList();
    }

    if (pageId === 'page-settings') {
        warmSettingsAssetCache();
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

    applyFallingPiecesPreference();
}

function renderAmbientBoardLayer() {
    const layer = document.getElementById('ambient-board-layer');
    if (!layer) return;

    const tileSize = window.innerWidth < 760 ? 72 : 96;
    const columns = Math.ceil(window.innerWidth / tileSize) + 10;
    const rows = Math.ceil(window.innerHeight / tileSize) + 10;
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
    if (!layer || ambientPieceIntervalId) return;

    for (let index = 0; index < 7; index += 1) {
        const timeoutId = window.setTimeout(() => spawnFallingPiece(layer), index * 360);
        ambientPieceTimeoutIds.push(timeoutId);
    }
    ambientPieceIntervalId = window.setInterval(() => spawnFallingPiece(layer), 850);
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
    const isSettingsPage = document.body.classList.contains('settings-page-active');
    if (settings.fallingPiecesEnabled && !reduceMotion && !isSettingsPage) {
        startFallingPieces();
        return;
    }
    stopFallingPieces();
}

function spawnFallingPiece(layer) {
    if (!settings.fallingPiecesEnabled) return;
    if (document.body.classList.contains('settings-page-active')) return;
    if (layer.childElementCount > 24) return;

    const pieceColor = Math.random() > 0.45 ? 'w' : 'b';
    const pieceType = PIECE_TYPES[Math.floor(Math.random() * PIECE_TYPES.length)];
    const piece = document.createElement('img');
    piece.className = 'falling-piece';
    piece.src = getPieceSrc(`${pieceColor}${pieceType}`);
    piece.alt = '';
    piece.style.setProperty('--fall-size', `${Math.round(42 + Math.random() * 42)}px`);
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
    if (mode === 'classic') return 'classic 8×8';
    if (mode === 'modern10') return 'modern 10×10';
    if (mode === 'modern12') return 'modern 12×12';
    return boardSize ? `${mode} ${boardSize}×${boardSize}` : mode;
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

    document.getElementById('classic-start-btn')?.addEventListener('click', async () => {
        if (!selectedClassicTimeMinutes) return;
        if (!await ensureMatchAuthentication()) return;
        renderClassicBoard(8, selectedClassicTimeMinutes, true, true, 'classic');
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

    document.getElementById('modern-start-btn')?.addEventListener('click', async () => {
        if (!selectedModernBoardSize || !selectedModernTimeMinutes) return;
        if (!await ensureMatchAuthentication()) return;
        const boardSize = selectedModernBoardSize;
        const timeControl = selectedModernTimeMinutes;
        navigateTo('page-classic');
        renderClassicBoard(boardSize, timeControl, true, true, modeForBoardSize(boardSize));
    });
}

function renderClassicSetupSelection() {
    document.querySelectorAll('[data-time-control]').forEach(button => {
        button.classList.toggle('active', Number(button.dataset.timeControl) === selectedClassicTimeMinutes);
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

    const startButton = document.getElementById('modern-start-btn');
    if (startButton) {
        startButton.disabled = !selectedModernBoardSize || !selectedModernTimeMinutes;
    }
}

function resetModernSetup() {
    selectedModernBoardSize = null;
    selectedModernTimeMinutes = null;
    renderModernSetupSelection();
}

function resetClassicEntry() {
    destroyBoard();
    cancelMatchmaking();
    stopGameTimer();
    hideMatchNotFoundOverlay();
    currentVisualBoardSize = null;
    currentTimeControlMinutes = null;
    currentGameMode = 'classic';
    currentCustomPosition = null;
    selectedCustomSquare = null;
    selectedClassicBoardSize = 8;
    selectedClassicTimeMinutes = null;
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

function renderClassicBoard(size, timeControlMinutes, resetPosition = false, restartSession = true, mode = currentGameMode) {
    const preservedClassicPosition = !resetPosition && size === 8 && board ? board.position() : null;
    destroyBoard();
    currentVisualBoardSize = size;
    currentTimeControlMinutes = timeControlMinutes;
    currentGameMode = mode;

    if (restartSession) {
        stopGameTimer();
        ensureEmojiChat();
        resetEmojiChatSession();
        capturedByMe = [];
        capturedByOpponent = [];
        renderCapturedPieces();
        renderPieceLegend();
        startMatchmaking(size, timeControlMinutes, mode);
        startGameTimer(timeControlMinutes);
    }

    document.getElementById('classic-setup')?.classList.add('hidden');
    document.getElementById('classic-board-shell')?.classList.remove('hidden');
    const label = document.getElementById('classic-board-size-label');
    if (label) {
        label.textContent = `${size}×${size} · ${timeControlMinutes} min`;
    }

    const host = document.getElementById('myBoard');
    if (!host) return;

    host.innerHTML = '';
    host.className = 'board-host';
    host.style.width = '';
    applySelectedBoardSquares();

    if (size === 8) {
        currentCustomPosition = null;
        host.style.width = 'var(--classic-board-size)';
        board = Chessboard('myBoard', {
            draggable: true,
            dropOffBoard: 'snapback',
            position: preservedClassicPosition || 'start',
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
        currentCustomPosition = buildVisualPosition(size);
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

function startMatchmaking(boardSize, timeControlMinutes, mode = currentGameMode) {
    setMatchmakingStatus('Searching...');
    activeMatchRequest = { mode, boardSize, timeControlMinutes };
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

    matchmakingClient.findMatch({ mode, boardSize, timeControlMinutes })
        .then(result => {
            if (!isCurrentMatchRequest(mode, boardSize, timeControlMinutes)) return;
            setMatchmakingStatus(result.message);
        })
        .catch(error => {
            if (!isCurrentMatchRequest(mode, boardSize, timeControlMinutes)) return;
            activeMatchRequest = null;
            queuedForMatch = false;
            pendingClassicMove = null;
            classicSnapbackInProgress = false;
            queuedClassicPositionUpdate = null;
            clearClassicMoveHighlights();
            stopGameTimer();
            setMatchmakingStatus(error.message || 'Unable to start matchmaking.');
        });
}

function cancelMatchmaking() {
    if (queuedForMatch) {
        matchmakingClient.cancel();
    }
    queuedForMatch = false;
    activeMatchRequest = null;
    pendingClassicMove = null;
    classicSnapbackInProgress = false;
    queuedClassicPositionUpdate = null;
    clearClassicMoveHighlights();
}

function isCurrentMatchRequest(mode, boardSize, timeControlMinutes) {
    return activeMatchRequest?.mode === mode
        && activeMatchRequest?.boardSize === boardSize
        && activeMatchRequest?.timeControlMinutes === timeControlMinutes;
}

function bindSocketEvents() {
    if (!window.ChessSocket) return;

    ChessSocket.on('QUEUE_JOINED', handleQueueJoined);
    ChessSocket.on('MATCH_FOUND', handleMatchFound);
    ChessSocket.on('GAME_STATE', handleGameState);
    ChessSocket.on('ERROR', handleSocketProtocolError);
    ChessSocket.on('MOVE_REJECTED', handleMoveRejected);
    ChessSocket.on('CLOSE', handleSocketClose);
}

function handleQueueJoined(payload) {
    queuedForMatch = true;
    const boardSize = payload?.board_size || activeMatchRequest?.boardSize || currentVisualBoardSize || 8;
    const mode = payload?.mode || activeMatchRequest?.mode || currentGameMode;
    const minutes = payload?.time_limit_minutes || activeMatchRequest?.timeControlMinutes || currentTimeControlMinutes;
    setMatchmakingStatus(`Searching for ${modeLabel(mode, boardSize)} · ${minutes} min.`);
}

function handleMatchFound(payload) {
    queuedForMatch = false;
    activeRemoteGame = true;
    currentGameState = null;
    currentGameId = payload?.game_id || null;
    currentPlayerColor = payload?.player_color || null;
    currentValidMoves = {};
    pendingClassicMove = null;
    classicSnapbackInProgress = false;
    queuedClassicPositionUpdate = null;
    clearClassicMoveHighlights();

    const boardSize = payload?.board_size || activeMatchRequest?.boardSize || currentVisualBoardSize || 8;
    const minutes = payload?.time_limit_minutes || activeMatchRequest?.timeControlMinutes || currentTimeControlMinutes || 10;
    const mode = payload?.mode || activeMatchRequest?.mode || currentGameMode;

    if (currentVisualBoardSize !== boardSize || currentGameMode !== mode) {
        renderClassicBoard(boardSize, minutes, true, false, mode);
    }

    const colorLabel = currentPlayerColor || 'unknown color';
    const opponent = payload?.opponent?.username || 'opponent';
    setMatchmakingStatus(`Match found vs ${opponent}. You play ${colorLabel}.`);
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

    const mode = activeMatchRequest?.mode || currentGameMode || modeForBoardSize(boardSize);
    const minutes = currentTimeControlMinutes || Math.max(
        Math.ceil((gameState.white_time_left_ms || 0) / 60000),
        Math.ceil((gameState.black_time_left_ms || 0) / 60000),
        1
    );

    ensureBoardForGameState(boardSize, minutes, mode);
    applyPositionFromGameState(gameState, boardSize, animateConfirmedClassicMove);
    pendingClassicMove = null;
    applyCapturedPiecesFromGameState(gameState);
    startServerGameTimer(gameState);

    const status = gameState.status || 'active';
    const turnLabel = status === 'active'
        ? (gameState.turn === currentPlayerColor ? 'Your turn' : 'Opponent turn')
        : 'Game finished';
    setMatchmakingStatus(`${status} · ${turnLabel}`);
}

function handleSocketProtocolError(payload) {
    const message = payload?.message || 'WebSocket error.';
    setMatchmakingStatus(message);

    if (payload?.code === 'UNKNOWN_MODE') {
        queuedForMatch = false;
        activeMatchRequest = null;
        currentGameState = null;
        currentValidMoves = {};
        pendingClassicMove = null;
        classicSnapbackInProgress = false;
        queuedClassicPositionUpdate = null;
        clearClassicMoveHighlights();
        stopGameTimer();
    }
}

function handleMoveRejected(payload) {
    pendingClassicMove = null;
    queuedClassicPositionUpdate = null;
    clearClassicMoveHighlights();
    setMatchmakingStatus(payload?.message || 'Move rejected.');
}

function handleSocketClose() {
    if (queuedForMatch) {
        queuedForMatch = false;
        activeMatchRequest = null;
        pendingClassicMove = null;
        classicSnapbackInProgress = false;
        queuedClassicPositionUpdate = null;
        clearClassicMoveHighlights();
        stopGameTimer();
        setMatchmakingStatus('Connection closed while searching.');
    }
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
    timerIntervalId = window.setInterval(tickGameTimer, 250);
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
        timerIntervalId = window.setInterval(tickGameTimer, 250);
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
    emojiMessages.push({
        id: createUserId('chat-me'),
        sender: 'me',
        name: accountProfile.signedIn && accountProfile.username ? accountProfile.username : 'Me',
        label: item.name,
        src: item.src
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
}

function renderHistoryList() {
    const list = document.getElementById('history-list');
    if (!list) return;

    const records = historyRecords
        .filter(record => historyFilters.size === 0 || historyFilters.has(record.result))
        .sort((a, b) => {
            const diff = new Date(a.timestamp) - new Date(b.timestamp);
            return historySortDirection === 'asc' ? diff : -diff;
        });

    list.innerHTML = '';
    if (records.length === 0) {
        const empty = document.createElement('div');
        empty.className = 'history-empty';
        empty.textContent = 'No games yet';
        list.appendChild(empty);
        return;
    }

    records.forEach(record => {
        const button = document.createElement('button');
        button.type = 'button';
        button.className = `history-game-card history-result-${record.result}`;
        button.addEventListener('click', () => openHistoryGame(record.id));

        const board = document.createElement('span');
        board.className = 'history-card-board';
        renderHistoryMiniBoard(board, record.boardSize);

        const meta = document.createElement('span');
        meta.className = 'history-card-meta';
        meta.innerHTML = `
            <strong>${resultLabel(record.result)} vs ${record.opponent}</strong>
            <span>${record.boardSize}×${record.boardSize} · ${record.timeControl}</span>
            <span>${formatHistoryDate(record.timestamp)}</span>
        `;

        button.append(board, meta);
        list.appendChild(button);
    });
}

function clearLocalHistoryRecords() {
    try {
        LEGACY_HISTORY_STORAGE_KEYS.forEach(key => localStorage.removeItem(key));
    } catch (error) {
        console.warn('Unable to clear local history records', error);
    }
}

function openHistoryGame(recordId) {
    const record = historyRecords.find(item => item.id === recordId) || historyRecords[0];
    if (!record) return;

    navigateTo('page-history-detail');
    const title = document.getElementById('history-detail-title');
    const accuracy = document.getElementById('history-accuracy');
    const result = document.getElementById('history-result');
    const opening = document.getElementById('history-opening');

    if (title) title.textContent = `Game vs ${record.opponent}`;
    if (accuracy) accuracy.textContent = record.accuracy;
    if (result) result.textContent = resultLabel(record.result);
    if (opening) opening.textContent = record.opening;

    renderHistoryAnalysisBoard(record);
    renderHistoryMoveList(record);
}

function renderHistoryMiniBoard(host, size) {
    host.innerHTML = '';
    for (let index = 0; index < 16; index += 1) {
        const square = document.createElement('span');
        square.className = (Math.floor(index / 4) + index) % 2 === 0 ? 'mini-light' : 'mini-dark';
        host.appendChild(square);
    }
    host.dataset.size = `${size}×${size}`;
}

function renderHistoryAnalysisBoard(record) {
    const host = document.getElementById('history-analysis-board');
    if (!host) return;

    host.innerHTML = '';
    const position = buildVisualPosition(8);
    const grid = document.createElement('div');
    grid.className = 'history-board-grid';

    for (let row = 0; row < 8; row += 1) {
        for (let col = 0; col < 8; col += 1) {
            const square = document.createElement('span');
            const key = squareKey(row, col);
            square.className = `history-board-square ${(row + col) % 2 === 0 ? 'mini-light' : 'mini-dark'}`;
            const piece = position[key];
            if (piece && (row < 2 || row > 5)) {
                const img = document.createElement('img');
                img.src = getPieceSrc(piece);
                img.alt = '';
                square.appendChild(img);
            }
            grid.appendChild(square);
        }
    }

    host.dataset.meta = `${record.boardSize}×${record.boardSize}`;
    host.appendChild(grid);
}

function renderHistoryMoveList(record) {
    const list = document.getElementById('history-move-list');
    if (!list) return;

    list.innerHTML = '';
    record.moves.forEach((move, index) => {
        const row = document.createElement('div');
        row.className = 'history-move-row';
        row.innerHTML = `<span>${index + 1}</span><strong>${move}</strong>`;
        list.appendChild(row);
    });
}

function resultLabel(result) {
    if (result === 'win') return 'Win';
    if (result === 'loss') return 'Loss';
    return 'Draw';
}

function formatHistoryDate(timestamp) {
    return new Date(timestamp).toLocaleDateString('en-US', {
        month: 'short',
        day: 'numeric',
        hour: '2-digit',
        minute: '2-digit'
    });
}

function renderEmojiMessages() {
    const log = document.getElementById('emoji-chat-log');
    if (!log) return;

    log.innerHTML = '';
    emojiMessages.forEach(message => {
        const row = document.createElement('div');
        row.className = `emoji-message ${message.sender}`;

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

function bindAccountForm() {
    const form = document.getElementById('account-form');
    const loginForm = document.getElementById('account-login-form');
    const profileAvatarInput = document.getElementById('account-profile-avatar-input');
    const refreshButton = document.getElementById('account-refresh-btn');
    const logoutButton = document.getElementById('account-logout-btn');

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

        try {
            setAccountFormsBusy(true);
            showAccountMessage('Creating account...');
            await ChessApi.register({ username, email, password });

            if (passwordInput) passwordInput.value = '';
            const loginEmail = document.getElementById('account-login-email');
            if (loginEmail) loginEmail.value = email;

            accountEditing = false;
            renderAccountProfile();
            showAccountMessage('Account created. Log in with your email and password.');
        } catch (error) {
            showAccountMessage(getAccountErrorMessage(error));
        } finally {
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
            accountEditing = false;
            renderAccountProfile();
            showAccountMessage('Logged in.');
        } catch (error) {
            if (error?.status === 401) {
                ChessApi.clearToken();
                accountProfile = createEmptyAccountProfile();
                renderAccountProfile();
            }
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
        window.ChessSocket?.close?.();
        ChessApi.logout();
        clearLegacyAccountProfile();
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
        accountEditing = false;
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
    const chip = document.getElementById('account-chip');
    const authPanel = document.getElementById('account-auth-panel');
    const loginForm = document.getElementById('account-login-form');
    const accountFormTitle = document.getElementById('account-form-title');
    const accountSubmitButton = document.getElementById('account-submit-btn');
    const profilePanel = document.getElementById('account-profile-panel');
    const profileName = document.getElementById('account-profile-name');
    const profileEmail = document.getElementById('account-profile-email');
    const profileRating = document.getElementById('account-profile-rating');

    const shouldShowProfile = accountProfile.signedIn;

    if (shouldShowProfile) {
        if (username) username.value = '';
        if (email) email.value = '';
        if (password) password.value = '';
        if (loginPassword) loginPassword.value = '';
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
    loginForm?.classList.toggle('hidden', shouldShowProfile);
    profilePanel?.classList.toggle('hidden', !shouldShowProfile);

    renderAccountAvatar('account-profile-avatar', 'account-profile-avatar-fallback', accountProfile.avatarSrc, accountInitial());

    if (profileName) profileName.textContent = accountProfile.username || 'Player';
    if (profileEmail) profileEmail.textContent = accountProfile.email || '-';
    if (profileRating) profileRating.textContent = accountProfile.rating || '-';
}

function createEmptyAccountProfile() {
    return {
        id: '',
        username: '',
        email: '',
        avatarSrc: '',
        rating: '-',
        registered: false,
        signedIn: false
    };
}

function applyBackendAccountProfile(profile) {
    accountProfile = {
        id: profile?.id || '',
        username: profile?.username || '',
        email: profile?.email || '',
        avatarSrc: accountProfile.avatarSrc || '',
        rating: profile?.rating ?? '-',
        registered: true,
        signedIn: true
    };
}

async function refreshAccountFromBackend({ showMessages = false } = {}) {
    if (!window.ChessApi?.hasToken?.()) {
        accountProfile = createEmptyAccountProfile();
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
            accountProfile = createEmptyAccountProfile();
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
    return window.ChessApi?.getErrorMessage?.(error) || error?.message || 'Unexpected account error.';
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
        setMatchmakingStatus('Waiting for game state from backend.');
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

    pendingClassicMove = { from: source, to: target };
    setMatchmakingStatus(`Sending move ${source}-${target}...`);

    try {
        if (window.ChessSocket?.move) {
            ChessSocket.move({ from: source, to: target });
        } else {
            ChessSocket.send('MOVE', { from: source, to: target });
        }
        classicSnapbackInProgress = true;
    } catch (error) {
        pendingClassicMove = null;
        classicSnapbackInProgress = false;
        queuedClassicPositionUpdate = null;
        setMatchmakingStatus(error.message || 'Unable to send move.');
    }

    return 'snapback';
}

function handleClassicSnapbackEnd() {
    classicSnapbackInProgress = false;
    clearClassicMoveHighlights();
    flushQueuedClassicPositionUpdate();
}

function canSubmitClassicMove(source, target, piece) {
    if (!isClassicBackendGameReady()) {
        setMatchmakingStatus('Waiting for game state from backend.');
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
        setMatchmakingStatus('WebSocket is not connected.');
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

function paintRenderedClassicSquares() {
    const light = getSquareStrategy(settings.lightSquareStrategyId);
    const dark = getSquareStrategy(settings.darkSquareStrategyId);

    document.querySelectorAll('#myBoard .white-1e1d7').forEach(square => {
        square.style.setProperty('background-color', light.getColor(), 'important');
        square.style.setProperty('background-image', `url("${light.getSrc()}")`, 'important');
        square.style.setProperty('background-size', 'cover', 'important');
        square.style.setProperty('background-position', 'center', 'important');
    });

    document.querySelectorAll('#myBoard .black-3c85d').forEach(square => {
        square.style.setProperty('background-color', dark.getColor(), 'important');
        square.style.setProperty('background-image', `url("${dark.getSrc()}")`, 'important');
        square.style.setProperty('background-size', 'cover', 'important');
        square.style.setProperty('background-position', 'center', 'important');
    });
}

function getPieceSrc(piece) {
    const strategyByType = piece[0] === 'w'
        ? settings.lightPieceStrategyByType
        : settings.darkPieceStrategyByType;
    const strategyId = strategyByType[piece[1]];
    return getPieceStrategy(strategyId).getSrc(piece);
}

function refreshCurrentBoard(resetPosition = false) {
    if (!currentVisualBoardSize || !currentTimeControlMinutes) return;
    renderClassicBoard(currentVisualBoardSize, currentTimeControlMinutes, resetPosition, false, currentGameMode);
}

function renderCustomBoard(host, size, position) {
    const lightSquare = getSquareStrategy(settings.lightSquareStrategyId);
    const darkSquare = getSquareStrategy(settings.darkSquareStrategyId);
    const grid = document.createElement('div');
    grid.className = 'custom-board';
    grid.dataset.size = String(size);
    grid.style.gridTemplateColumns = `repeat(${size}, minmax(0, 1fr))`;
    grid.style.gridTemplateRows = `repeat(${size}, minmax(0, 1fr))`;

    for (let row = 0; row < size; row += 1) {
        for (let col = 0; col < size; col += 1) {
            const square = document.createElement('div');
            const key = squareKey(row, col);
            const strategy = (row + col) % 2 === 0 ? lightSquare : darkSquare;

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

            appendCustomNotation(square, row, col, size);

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

function appendCustomNotation(square, row, col, size) {
    if (col === 0) {
        const rank = document.createElement('span');
        rank.className = 'custom-notation custom-numeric';
        rank.textContent = String(size - row);
        square.appendChild(rank);
    }

    if (row === size - 1) {
        const file = document.createElement('span');
        file.className = 'custom-notation custom-alpha';
        file.textContent = fileLabel(col);
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

    cancelCustomDrag();
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
            warmSettingsAssetCache();
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
            warmSettingsAssetCache();
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

function warmSettingsAssetCache() {
    const urls = new Set();

    getAllPieceStrategies().forEach(strategy => {
        PIECE_TYPES.forEach(type => {
            urls.add(strategy.getSrc(`w${type}`));
            urls.add(strategy.getSrc(`b${type}`));
        });
    });

    getAllSquareStrategies().forEach(strategy => {
        urls.add(strategy.getSrc());
    });

    urls.forEach(preloadAssetUrl);
}

function preloadAssetUrl(src) {
    if (!src || preloadedAssetImages.has(src)) return;

    const image = new Image();
    image.loading = 'eager';
    image.decoding = 'sync';
    image.fetchPriority = 'high';
    preloadedAssetImages.set(src, image);
    image.src = src;
    image.decode?.().catch(() => {});
}

function configurePreviewImage(image, src) {
    image.alt = '';
    image.loading = 'eager';
    image.decoding = 'sync';
    image.fetchPriority = 'high';
    preloadAssetUrl(src);
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
        throw new Error('Browser storage is full. Use smaller images for this test frontend.');
    }
}

function loadCurrentSettings() {
    const defaults = defaultSettings();

    try {
        const parsed = JSON.parse(localStorage.getItem(CURRENT_SETTINGS_KEY));
        if (!parsed) return defaults;

        const migratedPieceStyle = parsed.pieceStyleId || 'classic';
        const lightPieceStrategyByType = { ...defaults.lightPieceStrategyByType };
        const darkPieceStrategyByType = { ...defaults.darkPieceStrategyByType };
        PIECE_TYPES.forEach(type => {
            const legacyStrategyId = parsed.pieceStrategyByType?.[type] || parsed.pieceStyleByType?.[type] || migratedPieceStyle;
            lightPieceStrategyByType[type] = normalizeLoadedPieceStrategyId(parsed.lightPieceStrategyByType?.[type] || legacyStrategyId, type, 'light');
            darkPieceStrategyByType[type] = normalizeLoadedPieceStrategyId(parsed.darkPieceStrategyByType?.[type] || legacyStrategyId, type, 'dark');
        });

        return {
            lightPieceStrategyByType,
            darkPieceStrategyByType,
            lightSquareStrategyId: parsed.lightSquareStrategyId || parsed.lightSquareStyleId || parsed.boardStyleId || defaults.lightSquareStrategyId,
            darkSquareStrategyId: parsed.darkSquareStrategyId || parsed.darkSquareStyleId || parsed.boardStyleId || defaults.darkSquareStrategyId,
            backgroundStrategyId: parsed.backgroundStrategyId || defaults.backgroundStrategyId,
            fallingPiecesEnabled: typeof parsed.fallingPiecesEnabled === 'boolean' ? parsed.fallingPiecesEnabled : defaults.fallingPiecesEnabled
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
        fallingPiecesEnabled: true
    };
}

function normalizeSettings() {
    const pieceIds = getAllPieceStrategies().map(strategy => strategy.id);
    const squareIds = getAllSquareStrategies().map(strategy => strategy.id);
    const backgroundIds = backgroundStrategies.map(strategy => strategy.id);
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
        settings.fallingPiecesEnabled = true;
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
