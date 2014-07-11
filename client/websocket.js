var ProtoBuf = dcodeIO.ProtoBuf;
var ByteBuffer = ProtoBuf.ByteBuffer;
var builder;

var g_clientId;
var g_connectionManager;

function fnv32a(str)
{
    var FNV1_32A_INIT = 0x811c9dc5;
    var hval = FNV1_32A_INIT;
    for (var i = 0; i < str.length; ++i)
    {
        hval ^= str.charCodeAt(i);
        hval += (hval << 1) + (hval << 4) + (hval << 7) + (hval << 8) + (hval << 24);
    }
    return hval >>> 0;
}

// concatenate a list of ArrayBuffers into a single buffer
function concatBuffers(bufs) {
    var len = 0;
    _.each(bufs, function(e, i, l) { len += e.length; });

    var tmp = new Uint8Array(len);
    var ofs = 0;
    _.each(bufs, function(e, i, l) {
        tmp.set(new Uint8Array(e), ofs);
        ofs += e.length;
    });

    return tmp.buffer;
}

function ConnectionManager()
{
    this.websocket = null;
    this.nextToken = 0;
    this.RpcHeader = builder.build("swarm.Header");
    this.tokenToCallback = {}

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

    ConnectionManager.prototype.send = function(a)
    {
        this.websocket.send(a.toArrayBuffer());
    };

    ConnectionManager.prototype.sendProtoRequest = function(methodName, attr, cb)
    {
        var bb = new ByteBuffer();
        // build the header
        var name = 'swarm.' + methodName + 'Request';
        var hash = fnv32a(name);
        var token = this.nextToken;
        this.nextToken = this.nextToken + 1;

        var header = new this.RpcHeader({ 
            'method_hash' : hash,
            'token' : token,
            'is_response' : false});
        var headerBuf = header.toArrayBuffer();

        var Msg = builder.build(name);
        var body = new Msg(attr);
        var bodyBuf = body.toArrayBuffer();

        var buf = new ByteBuffer(2 + headerBuf.byteLength + bodyBuf.byteLength);

        // payload is: header_size + header + body
        buf.writeInt16(headerBuf.byteLength);
        buf.append(headerBuf);
        buf.append(bodyBuf);

        buf.reset()
        this.websocket.send(buf.toArrayBuffer());

        if (cb) {
            this.tokenToCallback[token] = { 'methodName' : methodName, 'callback' : cb };
        }
    }

    ConnectionManager.prototype.close = function()
    {
        this.websocket = null;
    }

    ConnectionManager.prototype.onConnect = function(evt)
    {
        var status = document.getElementById("connection-status");
        status.innerHTML = 'Connected';
        this.sendProtoRequest('Connection', {});
    }

    ConnectionManager.prototype.onClose = function(evt)
    {
    }

    ConnectionManager.prototype.onError = function(evt)
    {
        console.log(evt);
    }

    ConnectionManager.prototype.onMessage = function(evt)
    {
        var data = evt.data;
        // decode the header
        var header = this.RpcHeader.decode(data);

        if (header.is_response()) {

            // lookup callback, deserialize and dispatch
            var obj = this.tokenToCallback(header.token());
            if (obj) {
                // create a builder for the response object
                var name = 'swarm.' + obj.methodName + 'Response';
                var Msg = builder.build(name);
                var proto = Msg.decode(data);
                obj.callback(proto);
            }
            else
            {
                console.log('no callback found for token: ', header.token());
            }
        }

    }
}

function createProtobuf(msg, attr)
{
    var Msg = builder.build(msg);
    return new Msg(attr);
}

function createConnectionManager()
{
    g_connectionManager = new ConnectionManager();

}

function init(url) {
    builder = ProtoBuf.loadProtoFile("http://127.0.0.1:8000/protocol/swarm.proto");
    g_connectionManager = new ConnectionManager();
    g_connectionManager.connect(url);
}