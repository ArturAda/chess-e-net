const ASSET_ROOT = 'images';
const PIECES_ROOT = `${ASSET_ROOT}/сhess_pieces`;
const USER_STYLES_KEY = 'chessemag_user_styles';
const CURRENT_SETTINGS_KEY = 'chessemag_current_settings';
const ACCOUNT_PROFILE_KEY = 'chessemag_account_profile';
const ACCOUNT_AVATAR_SIZE = 256;
const TIMER_ROOT = `${ASSET_ROOT}/timer`;
const TIMER_DIGIT_ROOT = `${TIMER_ROOT}/digits`;

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
    constructor({ id, name, pieceType, whiteSrc, blackSrc }) {
        super({ id, name, pieceType });
        this.whiteSrc = whiteSrc;
        this.blackSrc = blackSrc;
    }

    getSrc(piece) {
        return piece[0] === 'w' ? this.whiteSrc : this.blackSrc;
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

class MatchmakingStrategy {
    findMatch() {
        throw new Error('MatchmakingStrategy#findMatch must be implemented.');
    }

    cancel() {
        throw new Error('MatchmakingStrategy#cancel must be implemented.');
    }
}

class LocalMatchmakingStrategy extends MatchmakingStrategy {
    findMatch({ mode, boardSize, timeControlMinutes }) {
        return Promise.resolve({
            mode,
            boardSize,
            timeControlMinutes,
            status: 'waiting',
            message: `Searching for ${mode} ${boardSize}×${boardSize}, ${timeControlMinutes} min match.`
        });
    }

    cancel() {
        return Promise.resolve({ status: 'cancelled' });
    }
}

const matchmakingClient = new LocalMatchmakingStrategy();

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

let board = null;
let currentVisualBoardSize = null;
let currentTimeControlMinutes = null;
let currentCustomPosition = null;
let selectedCustomSquare = null;
let selectedClassicBoardSize = null;
let selectedClassicTimeMinutes = null;
let timerState = null;
let timerIntervalId = null;
let matchNotFoundTimeoutId = null;
let settingsGalleryRendered = false;
let emojiChatRendered = false;
let emojiMessages = [];
let userStyles = loadUserStyles();
let settings = loadCurrentSettings();
let accountProfile = loadAccountProfile();
let accountPasswordVisible = false;
let accountEditing = false;

document.addEventListener('DOMContentLoaded', () => {
    normalizeSettings();
    bindClassicSetupControls();
    bindAccountForm();
    renderAccountProfile();
    setAccountEntryVisibility('page-menu');
    applySelectedBoardSquares();
});

window.addEventListener('resize', () => {
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

    if (pageId === 'page-classic') {
        resetClassicEntry();
    }

    if (pageId === 'page-settings') {
        renderSettingsGallery();
    }

    if (pageId === 'page-account') {
        accountEditing = false;
        accountPasswordVisible = false;
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

function bindClassicSetupControls() {
    document.querySelectorAll('[data-board-size]').forEach(button => {
        button.addEventListener('click', () => {
            selectedClassicBoardSize = Number(button.dataset.boardSize);
            renderClassicSetupSelection();
        });
    });

    document.querySelectorAll('[data-time-control]').forEach(button => {
        button.addEventListener('click', () => {
            selectedClassicTimeMinutes = Number(button.dataset.timeControl);
            renderClassicSetupSelection();
        });
    });

    document.getElementById('classic-start-btn')?.addEventListener('click', () => {
        if (!selectedClassicBoardSize || !selectedClassicTimeMinutes) return;
        renderClassicBoard(selectedClassicBoardSize, selectedClassicTimeMinutes, true);
    });
}

function renderClassicSetupSelection() {
    document.querySelectorAll('[data-board-size]').forEach(button => {
        button.classList.toggle('active', Number(button.dataset.boardSize) === selectedClassicBoardSize);
    });

    document.querySelectorAll('[data-time-control]').forEach(button => {
        button.classList.toggle('active', Number(button.dataset.timeControl) === selectedClassicTimeMinutes);
    });

    const startButton = document.getElementById('classic-start-btn');
    if (startButton) {
        startButton.disabled = !selectedClassicBoardSize || !selectedClassicTimeMinutes;
    }
}

function resetClassicEntry() {
    destroyBoard();
    cancelMatchmaking();
    stopGameTimer();
    hideMatchNotFoundOverlay();
    currentVisualBoardSize = null;
    currentTimeControlMinutes = null;
    currentCustomPosition = null;
    selectedCustomSquare = null;
    selectedClassicBoardSize = null;
    selectedClassicTimeMinutes = null;
    emojiMessages = [];
    renderClassicSetupSelection();
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

function renderClassicBoard(size, timeControlMinutes, resetPosition = false, restartSession = true) {
    const preservedClassicPosition = !resetPosition && size === 8 && board ? board.position() : null;
    destroyBoard();
    currentVisualBoardSize = size;
    currentTimeControlMinutes = timeControlMinutes;

    if (restartSession) {
        stopGameTimer();
        ensureEmojiChat();
        resetEmojiChatSession();
        startMatchmaking(size, timeControlMinutes);
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
            onDrop: handleClassicDrop
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
    if (board) {
        board.destroy();
        board = null;
    }
}

function startMatchmaking(boardSize, timeControlMinutes) {
    setMatchmakingStatus('Searching...');
    matchmakingClient.findMatch({ mode: 'classic', boardSize, timeControlMinutes }).then(result => {
        if (currentVisualBoardSize !== boardSize || currentTimeControlMinutes !== timeControlMinutes) return;
        setMatchmakingStatus(result.message);
    });
}

function cancelMatchmaking() {
    matchmakingClient.cancel();
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
    timerState.remaining.me = Math.max(0, timerState.remaining.me - elapsedSeconds);
    renderAllTimers(timerState.remaining.opponent, timerState.remaining.me);

    if (timerState.remaining.me === 0) {
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

    const initial = kind === 'me'
        ? timerState?.initialSeconds || 60
        : Math.max(seconds, 1);
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
    const passwordToggle = document.getElementById('account-password-toggle');
    const editButton = document.getElementById('account-edit-btn');
    const logoutButton = document.getElementById('account-logout-btn');

    form?.addEventListener('submit', event => {
        event.preventDefault();
        const username = document.getElementById('account-username')?.value.trim() || 'Player';
        const email = document.getElementById('account-email')?.value.trim() || '';
        const password = document.getElementById('account-password')?.value || (accountEditing ? accountProfile.password : '');
        if (!password) {
            showAccountMessage('Password is required.');
            return;
        }

        accountProfile = {
            username,
            email,
            password,
            avatarSrc: accountEditing ? accountProfile.avatarSrc || '' : '',
            rating: accountProfile.rating || '-',
            registered: true,
            signedIn: true
        };
        try {
            persistAccountProfile();
        } catch (error) {
            showAccountMessage(error.message);
            return;
        }
        accountPasswordVisible = false;
        accountEditing = false;
        renderAccountProfile();
        showAccountMessage('Account saved.');
    });

    loginForm?.addEventListener('submit', event => {
        event.preventDefault();
        const username = document.getElementById('account-login-username')?.value.trim() || '';
        const password = document.getElementById('account-login-password')?.value || '';

        if (!accountProfile.registered) {
            showAccountMessage('No local account is registered yet.');
            return;
        }

        if (username !== accountProfile.username || password !== accountProfile.password) {
            showAccountMessage('Username or password is incorrect.');
            return;
        }

        accountProfile.signedIn = true;
        persistAccountProfile();
        accountPasswordVisible = false;
        accountEditing = false;
        renderAccountProfile();
        showAccountMessage('Logged in.');
    });

    profileAvatarInput?.addEventListener('change', async event => {
        const [file] = Array.from(event.target.files || []);
        if (!file || !accountProfile.signedIn) return;

        try {
            accountProfile.avatarSrc = await readAccountAvatarAsDataUrl(file);
            persistAccountProfile();
            renderAccountProfile();
            showAccountMessage('Profile image saved locally.');
        } catch (error) {
            showAccountMessage(error.message);
        } finally {
            event.target.value = '';
        }
    });

    passwordToggle?.addEventListener('click', () => {
        accountPasswordVisible = !accountPasswordVisible;
        renderAccountProfile();
    });

    editButton?.addEventListener('click', () => {
        accountEditing = true;
        renderAccountProfile();
        showAccountMessage('');
    });

    logoutButton?.addEventListener('click', () => {
        accountProfile.signedIn = false;
        persistAccountProfile();
        accountPasswordVisible = false;
        accountEditing = false;
        renderAccountProfile();
        showAccountMessage('Logged out.');
    });
}

function loadAccountProfile() {
    try {
        const parsed = JSON.parse(localStorage.getItem(ACCOUNT_PROFILE_KEY));
        const username = parsed?.username || '';
        const password = parsed?.password || '';
        const registered = Boolean(parsed?.registered || (username && password));
        return {
            username,
            email: parsed?.email || '',
            password,
            avatarSrc: parsed?.avatarSrc || '',
            rating: parsed?.rating || '-',
            registered,
            signedIn: registered ? parsed?.signedIn ?? true : false
        };
    } catch {
        return createEmptyAccountProfile();
    }
}

function renderAccountProfile() {
    const username = document.getElementById('account-username');
    const email = document.getElementById('account-email');
    const password = document.getElementById('account-password');
    const loginUsername = document.getElementById('account-login-username');
    const loginPassword = document.getElementById('account-login-password');
    const chip = document.getElementById('account-chip');
    const authPanel = document.getElementById('account-auth-panel');
    const loginForm = document.getElementById('account-login-form');
    const accountFormTitle = document.getElementById('account-form-title');
    const accountSubmitButton = document.getElementById('account-submit-btn');
    const profilePanel = document.getElementById('account-profile-panel');
    const profileName = document.getElementById('account-profile-name');
    const profilePassword = document.getElementById('account-profile-password');
    const profileRating = document.getElementById('account-profile-rating');

    const shouldShowProfile = accountProfile.registered && accountProfile.signedIn && !accountEditing;
    const shouldShowLogin = accountProfile.registered && !accountProfile.signedIn && !accountEditing;
    const isEditing = accountProfile.registered && accountEditing;

    if (username) username.value = isEditing ? accountProfile.username || '' : '';
    if (email) email.value = isEditing ? accountProfile.email || '' : '';
    if (password) password.value = isEditing ? accountProfile.password || '' : '';
    if (loginUsername) loginUsername.value = shouldShowLogin ? accountProfile.username || '' : '';
    if (loginPassword) loginPassword.value = '';
    if (accountFormTitle) accountFormTitle.textContent = isEditing ? 'Edit Account' : 'Register';
    if (accountSubmitButton) accountSubmitButton.textContent = isEditing ? 'Save Profile' : 'Register';
    if (chip) {
        const label = accountProfile.signedIn ? `Account: ${accountProfile.username}` : 'Account';
        chip.title = label;
        chip.setAttribute('aria-label', label);
    }

    authPanel?.classList.toggle('hidden', shouldShowProfile);
    loginForm?.classList.toggle('hidden', !shouldShowLogin);
    profilePanel?.classList.toggle('hidden', !shouldShowProfile);

    renderAccountAvatar('account-profile-avatar', 'account-profile-avatar-fallback', accountProfile.avatarSrc, accountInitial());

    if (profileName) profileName.textContent = accountProfile.username || 'Player';
    if (profilePassword) {
        profilePassword.textContent = accountPasswordVisible
            ? accountProfile.password || '-'
            : maskPassword(accountProfile.password);
    }
    if (profileRating) profileRating.textContent = accountProfile.rating || '-';
}

function createEmptyAccountProfile() {
    return {
        username: '',
        email: '',
        password: '',
        avatarSrc: '',
        rating: '-',
        registered: false,
        signedIn: false
    };
}

function persistAccountProfile() {
    try {
        localStorage.setItem(ACCOUNT_PROFILE_KEY, JSON.stringify(accountProfile));
    } catch {
        throw new Error('Browser storage is full. Use a smaller profile image.');
    }
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

function maskPassword(password) {
    if (!password) return '-';
    return '*'.repeat(Math.max(8, password.length));
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

function handleClassicDrop(source, target) {
    if (!target || target === 'offboard' || source === target) return undefined;
    handleLocalMoveComplete();
    return undefined;
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
    const strategyId = settings.pieceStrategyByType[piece[1]];
    return getPieceStrategy(strategyId).getSrc(piece);
}

function refreshCurrentBoard(resetPosition = false) {
    if (!currentVisualBoardSize || !currentTimeControlMinutes) return;
    renderClassicBoard(currentVisualBoardSize, currentTimeControlMinutes, resetPosition, false);
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
                img.draggable = true;
                img.dataset.from = key;
                img.addEventListener('dragstart', handleCustomDragStart);
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

function handleCustomDragStart(event) {
    event.dataTransfer.setData('text/plain', event.currentTarget.dataset.from);
    event.dataTransfer.effectAllowed = 'move';
}

function handleCustomDrop(event) {
    event.preventDefault();
    if (!currentCustomPosition || !currentVisualBoardSize) return;

    const from = event.dataTransfer.getData('text/plain');
    const to = event.currentTarget.dataset.square;
    if (!from || !to || from === to || !currentCustomPosition[from]) return;

    currentCustomPosition[to] = currentCustomPosition[from];
    delete currentCustomPosition[from];
    selectedCustomSquare = null;
    refreshCurrentBoard(false);
    handleLocalMoveComplete();
}

function handleCustomSquareClick(event) {
    if (!currentCustomPosition) return;

    const target = event.currentTarget.dataset.square;
    if (!target) return;

    if (selectedCustomSquare && selectedCustomSquare !== target && currentCustomPosition[selectedCustomSquare]) {
        currentCustomPosition[target] = currentCustomPosition[selectedCustomSquare];
        delete currentCustomPosition[selectedCustomSquare];
        selectedCustomSquare = null;
        refreshCurrentBoard(false);
        handleLocalMoveComplete();
        return;
    }

    selectedCustomSquare = currentCustomPosition[target] ? target : null;
    refreshCurrentBoard(false);
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
    settingsGalleryRendered = true;
}

function createPieceSection(piece) {
    const selectedStrategyId = settings.pieceStrategyByType[piece.code];
    const section = createAssetSection({
        title: piece.title,
        iconSrc: getPieceStrategy(selectedStrategyId).getSrc(`w${piece.code}`)
    });
    const options = section.querySelector('.asset-options');

    getPieceStrategiesForType(piece.code).forEach(strategy => {
        options.appendChild(createPieceOption(piece.code, strategy, selectedStrategyId));
    });
    options.appendChild(createPieceUploadOption(piece));

    return section;
}

function createPieceOption(pieceType, strategy, selectedStrategyId) {
    const option = document.createElement('button');
    option.type = 'button';
    option.className = `asset-option ${strategy.id === selectedStrategyId ? 'active' : ''}`;
    option.addEventListener('click', () => selectPieceStrategy(pieceType, strategy.id));

    const pair = document.createElement('span');
    pair.className = 'asset-piece-pair';
    ['w', 'b'].forEach(color => {
        const img = document.createElement('img');
        img.src = strategy.getSrc(`${color}${pieceType}`);
        img.alt = '';
        pair.appendChild(img);
    });

    const name = document.createElement('span');
    name.className = 'asset-option-name';
    name.textContent = strategy.name;

    option.append(pair, name);
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

    const fileInput = document.createElement('input');
    fileInput.type = 'file';
    fileInput.accept = '.png,.jpg,.jpeg,.svg';
    fileInput.multiple = true;
    fileInput.addEventListener('change', async event => {
        const files = Array.from(event.target.files || []);
        if (!files.length) return;

        try {
            const variant = await createUserPieceVariant(piece.code, files, nameInput.value.trim());
            userStyles.pieceVariants.push(variant);
            persistUserStyles();
            selectPieceStrategy(piece.code, variant.id);
            showSettingsMessage(`Piece variant saved: ${variant.name}`);
        } catch (error) {
            showSettingsMessage(error.message);
        } finally {
            event.target.value = '';
        }
    });

    const label = document.createElement('label');
    label.className = 'inline-upload-btn';
    label.append(plus, document.createTextNode(`Upload ${piece.title}`), fileInput);

    wrapper.append(label, nameInput);
    return wrapper;
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
        createSquareRoleControl('Light', strategy.id, lightId, () => selectSquareStrategy('light', strategy.id)),
        createSquareRoleControl('Dark', strategy.id, darkId, () => selectSquareStrategy('dark', strategy.id))
    );

    option.append(swatch, name, controls);
    return option;
}

function createSquareRoleControl(labelText, strategyId, selectedId, onSelect) {
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
    fileInput.accept = '.png,.jpg,.jpeg,.svg';
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

function createAssetSection({ title, iconSrc, squareSrc }) {
    const section = document.createElement('section');
    section.className = 'asset-section';

    const header = document.createElement('div');
    header.className = 'asset-header';

    if (iconSrc) {
        const icon = document.createElement('img');
        icon.className = 'asset-icon';
        icon.src = iconSrc;
        icon.alt = '';
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

function selectPieceStrategy(pieceType, strategyId) {
    settings.pieceStrategyByType[pieceType] = strategyId;
    saveCurrentSettings();
    if (settingsGalleryRendered) {
        renderSettingsGallery();
    }
    refreshCurrentBoard(false);
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

async function createUserPieceVariant(pieceType, files, requestedName) {
    const sources = await Promise.all(files.slice(0, 2).map(readFileAsDataUrl));
    let whiteSrc = sources[0];
    let blackSrc = sources[1] || sources[0];

    const whiteFile = files.find(file => /white|light|_w|(^|[-_])w[-_]?/i.test(file.name));
    const blackFile = files.find(file => /black|dark|_b|(^|[-_])b[-_]?/i.test(file.name));
    if (whiteFile) whiteSrc = await readFileAsDataUrl(whiteFile);
    if (blackFile) blackSrc = await readFileAsDataUrl(blackFile);

    return {
        id: createUserId(`piece-${pieceType.toLowerCase()}`),
        name: requestedName || `Custom ${PIECE_NAMES[pieceType]}`,
        pieceType,
        whiteSrc,
        blackSrc
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
        ...builtInPieceStrategies,
        ...userStyles.pieceVariants.map(variant => new UploadedPieceVariantStrategy(variant))
    ];
}

function getPieceStrategiesForType(pieceType) {
    return getAllPieceStrategies().filter(strategy => !strategy.pieceType || strategy.pieceType === pieceType);
}

function getPieceStrategy(strategyId) {
    return getAllPieceStrategies().find(strategy => strategy.id === strategyId) || builtInPieceStrategies[0];
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

function loadUserStyles() {
    try {
        const parsed = JSON.parse(localStorage.getItem(USER_STYLES_KEY));
        return normalizeUserStyles(parsed);
    } catch {
        return normalizeUserStyles();
    }
}

function normalizeUserStyles(parsed = {}) {
    return {
        pieceVariants: Array.isArray(parsed.pieceVariants) ? parsed.pieceVariants : migrateOldPieceVariants(parsed.pieces),
        squareVariants: Array.isArray(parsed.squareVariants) ? parsed.squareVariants : migrateOldSquareVariants(parsed.boards)
    };
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
        const pieceStrategyByType = { ...defaults.pieceStrategyByType };
        PIECE_TYPES.forEach(type => {
            pieceStrategyByType[type] = parsed.pieceStrategyByType?.[type] || parsed.pieceStyleByType?.[type] || migratedPieceStyle;
        });

        return {
            pieceStrategyByType,
            lightSquareStrategyId: parsed.lightSquareStrategyId || parsed.lightSquareStyleId || parsed.boardStyleId || defaults.lightSquareStrategyId,
            darkSquareStrategyId: parsed.darkSquareStrategyId || parsed.darkSquareStyleId || parsed.boardStyleId || defaults.darkSquareStrategyId
        };
    } catch {
        return defaults;
    }
}

function defaultSettings() {
    return {
        pieceStrategyByType: PIECE_TYPES.reduce((result, type) => {
            result[type] = builtInPieceStrategies[0].id;
            return result;
        }, {}),
        lightSquareStrategyId: 'yellow-square',
        darkSquareStrategyId: 'classic-green-square'
    };
}

function normalizeSettings() {
    const pieceIds = getAllPieceStrategies().map(strategy => strategy.id);
    const squareIds = getAllSquareStrategies().map(strategy => strategy.id);

    PIECE_TYPES.forEach(type => {
        if (!pieceIds.includes(settings.pieceStrategyByType[type])) {
            settings.pieceStrategyByType[type] = builtInPieceStrategies[0].id;
        }
    });

    if (!squareIds.includes(settings.lightSquareStrategyId)) {
        settings.lightSquareStrategyId = 'yellow-square';
    }

    if (!squareIds.includes(settings.darkSquareStrategyId)) {
        settings.darkSquareStrategyId = 'classic-green-square';
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
