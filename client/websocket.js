var ProtoBuf    = dcodeIO.ProtoBuf;
var ByteBuffer  = ProtoBuf.ByteBuffer;
var g_builder;
var Vector2;

var g_clientId;
var g_connectionManager;
var g_playerId;
var g_gameId;

var g_game;

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
                g_game = new Phaser.Game(1800, 600, Phaser.AUTO, '', 
                    { preload: preload, create: create, update: update });

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
    g_builder           = ProtoBuf.loadProtoFile("protocol/swarm.proto");
    Vector2             = g_builder.build("swarm.Vector2");
    g_connectionManager = new ConnectionManager();

    g_connectionManager.addMethodHandler('EnterGame', function(header, body) { 
        console.log(header, body);
        g_gameId = body.gameId;
        g_playerId = body.playerId;
    })

    g_connectionManager.addMethodHandler('PingRequest', function(header, body) { 
//        console.log(header, body); });    
        g_connectionManager.sendProtoResponse('Ping', header.token, {})
    })
    g_connectionManager.connect(url);
}

function disconnect() {
    g_connectionManager.disconnect();
}

function preload() {

    g_game.load.image('sky', 'assets/sky.png');
    g_game.load.image('ground', 'assets/platform.png');
    g_game.load.image('star', 'assets/star.png');
    g_game.load.spritesheet('dude', 'assets/dude.png', 32, 48);
}

function create() {
    //  We're going to be using physics, so enable the Arcade Physics system
    g_game.physics.startSystem(Phaser.Physics.ARCADE);
 
    //  A simple background for our game
    g_game.add.sprite(0, 0, 'sky');
 
    //  The platforms group contains the ground and the 2 ledges we can jump on
    platforms = g_game.add.group();
 
    //  We will enable physics for any object that is created in this group
    platforms.enableBody = true;
 
    // Here we create the ground.
    var ground = platforms.create(0, g_game.world.height - 64, 'ground');
 
    //  Scale it to fit the width of the game (the original sprite is 400x32 in size)
    ground.scale.setTo(2, 2);
 
    //  This stops it from falling away when you jump on it
    ground.body.immovable = true;
 
    //  Now let's create two ledges
    var ledge = platforms.create(400, 400, 'ground');
 
    ledge.body.immovable = true;
 
    ledge = platforms.create(-150, 250, 'ground');
 
    ledge.body.immovable = true;

    // The player and its settings
    player = g_game.add.sprite(32, g_game.world.height - 450, 'dude');
 
    //  We need to enable physics on the player
    g_game.physics.arcade.enable(player);
 
    //  Player physics properties. Give the little guy a slight bounce.
    player.body.bounce.y = 0.2;
    player.body.gravity.y = 300;
    player.body.collideWorldBounds = true;
 
    //  Our two animations, walking left and right.
    player.animations.add('left', [0, 1, 2, 3], 10, true);
    player.animations.add('right', [5, 6, 7, 8], 10, true);

    stars = g_game.add.group();

    stars.enableBody = true;

    //  Here we'll create 12 of them evenly spaced apart
    for (var i = 0; i < 12; i++)
    {
        //  Create a star inside of the 'stars' group
        var star = stars.create(i * 70, 0, 'star');

        //  Let gravity do its thing
        star.body.gravity.y = 20;

        //  This just gives each star a slightly random bounce value
        star.body.bounce.y = 0.7 + Math.random() * 0.2;
    }

    cursors = g_game.input.keyboard.createCursorKeys();
}

function update() {

        //  Reset the players velocity (movement)
    player.body.velocity.x *= 0.98;
    player.body.acceleration.x = 0;

    if (cursors.left.isDown)
    {
        //  Move to the left
        player.body.acceleration.x = -150;

        player.animations.play('left');
    }
    else if (cursors.right.isDown)
    {
        //  Move to the right
        player.body.acceleration.x = 150;

        player.animations.play('right');
    }
    else
    {
        //  Stand still
        player.animations.stop();

        player.frame = 4;
    }
    
    //  Allow the player to jump if they are touching the ground.
    if (cursors.up.isDown && player.body.touching.down)
    {
        player.body.acceleration.y = -350;
    }

    g_game.physics.arcade.collide(stars, platforms);

    //  Collide the player and the stars with the platforms
    g_game.physics.arcade.collide(player, platforms);

    g_connectionManager.sendProto('PlayerState', {
        'id' : g_playerId,
        'acc' : new Vector2({'x': player.body.acceleration.x, 'y': player.body.acceleration.y}),
        'vel' : new Vector2({'x': player.body.velocity.x, 'y': player.body.velocity.y}),
        'pos' : new Vector2({'x': player.body.position.x, 'y': player.body.position.y}),
        'time' : g_game.time.now,
    });

}