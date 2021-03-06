var ProtoBuf    = dcodeIO.ProtoBuf;
var ByteBuffer  = ProtoBuf.ByteBuffer;
var g_builder;
var Vector2;

var g_clientId = -1;
var g_connectionManager = null;

var g_game = null;
var g_players = {}
var g_playerId = -1
var g_gameId;

var g_preloadDone = false;
var g_createDone = false;
var g_lastSend = 0;

var g_collisionItem = null;

function reset() {
    g_clientId = -1;
    g_connectionManager = null;
    g_game = null;

    g_players = {};
    g_playerId = -1;
    g_gameId = -1;

    g_preloadDone = false;
    g_createDone = false;
    g_lastSend = 0;

    g_collisionItem = null;
}

function ConnectionManager()
{
    this.websocket = null;
    this.nextToken = 0;
    this.RpcHeader = g_builder.build("swarm.Header");
    this.tokenToCallback = {}
    this.builderByName = {}
    this.builderByHash = {}
    this.methodHandlers = {}

    ConnectionManager.prototype.connect = function(url)
    {
        this.websocket = new WebSocket(url);
        this.websocket.binaryType = 'arraybuffer';

        var that = this;

        this.websocket.onopen       = function(e) { that.onConnect(e); }
        this.websocket.onclose      = function(e) { that.onClose(e); }
        this.websocket.onmessage    = function(e) { that.onMessage(e); }
        this.websocket.onerror      = function(e) { that.onError(e); }
    };

    ConnectionManager.prototype.disconnect = function() {
        this.websocket.close();
    };

    ConnectionManager.prototype.send = function(a)
    {
        this.websocket.send(a.toArrayBuffer());
    };

    ConnectionManager.prototype.createBuilder = function(name) {

        // check the cache
        var o;
        if (name in this.builderByName) {
            o = this.builderByName[name];
        } else {
            o = g_builder.build(name);
            this.builderByName[name] = o;
            this.builderByHash[UTILS.fnv32a(name)] = o;
        }
        return o;
    };

    ConnectionManager.prototype.addMethodHandler = function(methodName, cb)
    {
        var fullName = 'swarm.' + methodName;

        // create builder, and add it to the cache
        this.createBuilder(fullName);

        // register callback
        var methodHash = UTILS.fnv32a(fullName);
        this.methodHandlers[methodHash] = cb;

        console.log('Added handler for: ', fullName, ' - ', methodHash);
    };

    ConnectionManager.prototype.createArrayBuffer = function(header, name, attr) {

        var headerBuf = header.toArrayBuffer();

        var Msg = this.createBuilder(name);

        var buf;
        if (attr) {
            var body = new Msg(attr);
            var bodyBuf = body.toArrayBuffer();

            var buf = new ByteBuffer(2 + headerBuf.byteLength + bodyBuf.byteLength);

            // payload is: header_size + header + body
            buf.writeInt16(headerBuf.byteLength);
            buf.append(headerBuf);
            buf.append(bodyBuf);
        } else {
            var buf = new ByteBuffer(2 + headerBuf.byteLength);

            // payload is: header_size + header
            buf.writeInt16(headerBuf.byteLength);
            buf.append(headerBuf);
        }

        buf.reset()
        return buf.toArrayBuffer();
    }

    // todo: unify these guys..
    ConnectionManager.prototype.sendProto = function(methodName, attr)
    {
        var bb = new ByteBuffer();
        // build the header
        var name = 'swarm.' + methodName;
        var token = this.nextToken;
        var header = new this.RpcHeader({ 
            'method_hash' : UTILS.fnv32a(name),
            'token' : token,
            'is_response' : false});
        this.nextToken = token + 1;
        this.websocket.send(this.createArrayBuffer(header, name, attr));
    }

    ConnectionManager.prototype.sendProtoRequest = function(methodName, attr, cb)
    {
        var bb = new ByteBuffer();
        // build the header
        var name = 'swarm.' + methodName + 'Request';
        var token = this.nextToken;
        var header = new this.RpcHeader({ 
            'method_hash' : UTILS.fnv32a(name),
            'token' : token,
            'is_response' : false});
        this.nextToken = token + 1;

        this.websocket.send(this.createArrayBuffer(header, name, attr));

        if (cb) {
            this.tokenToCallback[token] = { 'methodName' : methodName, 'callback' : cb };
        }
    }

    ConnectionManager.prototype.sendProtoResponse = function(methodName, token, attr)
    {
        var bb = new ByteBuffer();
        // build the header
        var name = 'swarm.' + methodName + 'Response';
        var header = new this.RpcHeader({ 
            'method_hash' : UTILS.fnv32a(name),
            'token' : token,
            'is_response' : true});

        this.websocket.send(this.createArrayBuffer(header, name, attr));
    }

    ConnectionManager.prototype.onConnect = function(evt)
    {
        var status = document.getElementById("connection-status");
        status.innerHTML = 'Connected';
        this.sendProtoRequest('Connection', {'create_game' : true}, 
            function(x) { 
                console.log(x); 
            });
    }

    ConnectionManager.prototype.onClose = function(evt)
    {
        this.websocket = null;
    }

    ConnectionManager.prototype.onError = function(evt)
    {
        console.log(evt);
    }

    ConnectionManager.prototype.onMessage = function(evt)
    {
        var data = evt.data;

        // get header length
        var bb = ByteBuffer.wrap(data);
        var headerSize = bb.readUint16();

        var bbHeader = bb.copy(2, 2+headerSize);
        var bbBody = bb.copy(2+headerSize, data.byteLength)

        // decode the header
        var header = this.RpcHeader.decode(bbHeader.toArrayBuffer());

        if (header.is_response) {

            // lookup callback, deserialize and dispatch
            var obj = this.tokenToCallback[header.token];
            if (obj) {
                // create a builder for the response object
                var name = 'swarm.' + obj.methodName + 'Response';
                var Msg = this.createBuilder(name);
                var proto = Msg.decode(bbBody.toArrayBuffer());
                obj.callback(header, proto);
            }
            else
            {
                console.log('no callback found for token: ', header.token);
            }
        }
        else {
            var methodHash = header.method_hash;
            var cb = this.methodHandlers[methodHash];
            if (cb) {
                // find the builder, and decode the payload
                var Msg = this.builderByHash[methodHash];
                if (Msg) {
                    var proto = Msg.decode(bbBody.toArrayBuffer());
                    cb(header, proto);
                } else {
                    console.log('Builder not found for: ', header);
                }

            } else {
                console.log('Unhandled method: ', header)
            }
        }

    }
}

function createProtobuf(msg, attr)
{
    var Msg = g_builder.build(msg);
    return new Msg(attr);
}

function createConnectionManager()
{
    g_connectionManager = new ConnectionManager();
}

function init(url) {
    reset();
    g_builder           = ProtoBuf.loadProtoFile("protocol/swarm.proto");
    Vector2             = g_builder.build("swarm.Vector2");
    g_connectionManager = new ConnectionManager();

    g_connectionManager.addMethodHandler('EnterGame', function(header, body) { 
        console.log('** ENTER GAME', body)
        g_gameId = body.gameId
        g_playerId = body.playerId
        g_game = new Phaser.Game(1800, 600, Phaser.AUTO, '', { 
                    preload: preload,
                    create: function() { 
                        create(body.players);
                    },
                    update: update });
    });

    g_connectionManager.addMethodHandler('GameState', function(header, body) { 
        _.each(body.players, function(v, k, l) {

            // ignore updates if the game isn't created yet
            if (!g_createDone)
                return;

            if (!(v.id in g_players)) {
                createPlayer(v.id, v.id == g_playerId);
                console.log('[GameState] Added new player', v, g_players)
            }

            if (v.id != g_playerId) {
                var player = g_players[v.id];
                player.position.x = v.pos.x;
                player.position.y = v.pos.y;
            }
        })
    });

    g_connectionManager.addMethodHandler('PingRequest', function(header, body) { 
        g_connectionManager.sendProtoResponse('Ping', header.token, {})
    })
    
    g_connectionManager.connect(url);
}

function disconnect() {
    g_connectionManager.disconnect();
}

function preload() {

    g_preloadDone = false;
    g_game.load.image('sky', 'assets/sky.png');
    g_game.load.image('ground', 'assets/platform.png');
    g_game.load.image('star', 'assets/star.png');
    g_game.load.image('firstaid', 'assets/firstaid.png');
    g_game.load.spritesheet('dude', 'assets/dude.png', 32, 48);
    g_preloadDone = true;
}

function createPlayer(id, local) {

    console.log('[createPlayer]: ', id, local)
    // The player and its settings
    var player = g_game.add.sprite(32, g_game.world.height - 450, 'dude');
 
    if (local) {

        if (!g_preloadDone) {
            console.log('** CREATE PLAYER BEFORE PRELOAD');
        }

        if (!g_createDone) {
            console.log('** CREATE PLAYER BEFORE GAME CREATE');
        }

        //  We need to enable physics on the player
        g_game.physics.arcade.enable(player);
     
        //  Player physics properties. Give the little guy a slight bounce.
        player.body.bounce.y = 0.2;
//        player.body.gravity.y = 300;
        player.body.collideWorldBounds = true;
     
        //  Our two animations, walking left and right.
        player.animations.add('left', [0, 1, 2, 3], 10, true);
        player.animations.add('right', [5, 6, 7, 8], 10, true);        
    }

    g_players[id] = player;
}

g_level1 = {
    width : 10,
    height : 10,
    data : [
        'xxxxxxxxxx',
        'x  1     x',
        'x   2    x',
        'x        x',
        'x        x',
        'x        x',
        'x        x',
        'x        x',
        'x        x',
        'xxxxxxxxxx'
    ],
};

g_buttons = []

function collision1(player, obj) {
//    console.log('collision: ', player, obj)
 }

function collision2(player, obj) {
    g_collisionItem = obj;
//    console.log('collision: ', player, obj)
//    obj.kill()
 }

function createLevel(game, level) {

    platforms = game.add.group();
    platforms.enableBody = true;

    buttons = game.add.group();
    buttons.enableBody = true;

    var w = level.width;
    var h = level.height;

    var blockSize = 30;

    _.each(level.data, function(cur, y, arr) {
        _.each(cur, function(elem, x, row) {
            if (elem == 'x') {
                var block = platforms.create(blockSize*x, blockSize*y, 'star');
                block.body.immovable = true;

           } else if (elem == '1') {
                var block = buttons.create(blockSize*x, blockSize*y, 'firstaid');
                g_buttons.push({obj: block, col: collision1});

           } else if (elem == '2') {
                var block = buttons.create(blockSize*x, blockSize*y, 'firstaid');
                g_buttons.push({obj: block, col: collision2});
           }
        })
    })
}

function create(players) {
    g_createDone = false;

    //  We're going to be using physics, so enable the Arcade Physics system
    g_game.physics.startSystem(Phaser.Physics.ARCADE);
 
    //  A simple background for our game
    g_game.add.sprite(0, 0, 'sky');

    createLevel(g_game, g_level1);
 
    var keyboard = g_game.input.keyboard;
    cursors = keyboard.createCursorKeys();
    keyboard.addKeyCapture(Phaser.Keyboard.SPACEBAR);
    var space = keyboard.addKey(Phaser.Keyboard.SPACEBAR);
    space.onUp.add(onSpace, g_players);
}

function onSpace() {
    // check if the player should activate any item
    if (g_collisionItem) {
        console.log('activate item');
    }
}

function update() {

    g_collisionItem = null;

    if (g_playerId != -1 && (g_playerId in g_players)) {
        // update the local player with keyboard input
        var player = g_players[g_playerId];

        player.body.velocity.x *= 0.98;
        player.body.velocity.y *= 0.98;
        player.body.acceleration.x = 0;
        player.body.acceleration.y = 0;

        var moving = false;
        if (cursors.left.isDown) {
            player.body.acceleration.x = -150;
            player.animations.play('left');
            moving = true;

        } else if (cursors.right.isDown) {
            player.body.acceleration.x = 150;
            player.animations.play('right');
            moving = true;
        }

        if (cursors.up.isDown) {
            player.body.acceleration.y = -150;
            moving = true;

        } else if (cursors.down.isDown) {
            player.body.acceleration.y = 150;
            moving = true;
        }

        if (!moving) {
            //  Stand still
            player.animations.stop();
            player.frame = 4;
        }
        
        //  Collide the player and the stars with the platforms
        g_game.physics.arcade.collide(player, platforms);

        var now = g_game.time.now;
        if (now - g_lastSend > 250) {
            g_connectionManager.sendProto('PlayerState', {
                'id' : g_playerId,
                'acc' : new Vector2({'x': player.body.acceleration.x, 'y': player.body.acceleration.y}),
                'vel' : new Vector2({'x': player.body.velocity.x, 'y': player.body.velocity.y}),
                'pos' : new Vector2({'x': player.body.position.x, 'y': player.body.position.y}),
                'time' : g_game.time.now,
            });
            g_lastSend = now;
        }

        _.each(g_buttons, function(cur, idx, arr) {
            g_game.physics.arcade.overlap(player, cur.obj, cur.col, null, this);
        })
    }   
    
//    g_game.physics.arcade.collide(stars, platforms);
    g_createDone = true;
}