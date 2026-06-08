const ASSET_ROOT = 'images';
const PIECES_ROOT = `${ASSET_ROOT}/сhess_pieces`;
const USER_STYLES_KEY = 'chessemag_user_styles';
const USER_STYLES_VERSION = 2;
const CURRENT_SETTINGS_KEY = 'chessemag_current_settings';
const CURRENT_SETTINGS_VERSION = 2;
const LEGACY_ACCOUNT_PROFILE_KEY = 'chessemag_account_profile';
const ACCOUNT_AVATAR_SIZE = 256;
const TIMER_ROOT = `${ASSET_ROOT}/timer`;
const TIMER_DIGIT_ROOT = `${TIMER_ROOT}/digits`;
const GAME_TIMER_TICK_MS = 1000;
const ACCOUNT_VERIFICATION_SECONDS = 60;
const CUSTOM_DRAG_START_THRESHOLD = 5;
const CUSTOM_DRAG_CLICK_SUPPRESS_MS = 250;
const FALLING_PIECE_INITIAL_COUNT = 3;
const FALLING_PIECE_INTERVAL_MS = 1800;
const FALLING_PIECE_MAX_COUNT = 10;
const GAME_GENERIC_ERROR_MESSAGE = 'Something went wrong. Try again.';
const GAME_CONNECTION_LOST_MESSAGE = 'Game connection was lost. Start search again.';
const GAME_SETUP_PENDING_MESSAGE = 'The board is still setting up.';
const GAME_TECHNICAL_MESSAGE_PATTERN = /\b(backend|websocket|jwt|localhost|authorization bearer|token|payload|json|queue manager|request failed|frontend|server error|message type|board_size)\b/i;
const SOCKET_ERROR_MESSAGES = {
    SOCKET_ERROR: 'Could not enter the match room. Try again.',
    INVALID_SERVER_MESSAGE: 'The match room sent an unexpected update. Refresh the page if this repeats.',
    INVALID_MESSAGE: 'The game did not understand that action. Try again.',
    UNKNOWN_MESSAGE: 'This game action is not supported yet.',
    UNKNOWN_MODE: 'This board mode is not available right now.',
    QUEUE_FAILED: 'Matchmaking is unavailable right now. Try again soon.',
    NOT_IN_GAME: 'This game is no longer active.',
    NOT_YOUR_TURN: 'Wait for your turn.',
    INVALID_MOVE: 'That move is not legal.',
    GAME_ALREADY_OVER: 'This game is already finished.',
    DRAW_OFFER_ACTIVE: 'A draw offer is already active.',
    DRAW_OFFER_STATE: 'This draw offer is no longer available.',
    INVALID_STICKER: 'This sticker is not available.',
    INTERNAL_ERROR: 'The match room is busy right now. Try again soon.'
};

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

function playerFacingGameMessage(message, fallback = GAME_GENERIC_ERROR_MESSAGE) {
    const text = String(message || '').trim();
    if (!text || GAME_TECHNICAL_MESSAGE_PATTERN.test(text)) {
        return fallback;
    }
    return text;
}

function playerFacingErrorMessage(error, fallback = GAME_GENERIC_ERROR_MESSAGE) {
    return playerFacingGameMessage(error?.message, fallback);
}

function playerFacingSocketMessage(payload, fallback = GAME_GENERIC_ERROR_MESSAGE) {
    const code = payload?.code || '';
    if (SOCKET_ERROR_MESSAGES[code]) {
        return SOCKET_ERROR_MESSAGES[code];
    }
    return playerFacingGameMessage(payload?.message, fallback);
}

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
    async findMatch({ mode, boardSize, timeControlMinutes, isRanked = false }) {
        const token = window.ChessApi?.getToken?.();
        if (!token) {
            throw new Error('Log in before searching for a match.');
        }

        if (!window.ChessSocket) {
            throw new Error('Game connection is still loading. Refresh the page and try again.');
        }

        await ChessSocket.connect(token);
        ChessSocket.joinQueue({
            mode,
            boardSize,
            isRanked,
            timeControlMinutes,
            visualState: buildCurrentVisualStatePayload()
        });

        return {
            mode,
            boardSize,
            timeControlMinutes,
            isRanked,
            status: 'waiting',
            message: `Searching for ${modeLabel(mode, boardSize)} · ${timeControlMinutes} min · ${isRanked ? 'ranked' : 'casual'}.`
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
