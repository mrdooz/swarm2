var ProtoBuf = dcodeIO.ProtoBuf;
var ByteBuffer = ProtoBuf.ByteBuffer;
var builder;
var wsUri = "ws://127.0.0.1:8080/";
var output;

var p = "package connection; message ConnectionResponse { optional uint32 connectionId = 1; }";


function init() {
    console.log("test");
    output = document.getElementById("output");
//    builder = ProtoBuf.loadProtoFile("http://127.0.0.1:8000/protocol/connection.proto");
    builder = ProtoBuf.loadProto(p);
    testWebSocket();
}

function testWebSocket() {
    websocket = new WebSocket(wsUri);
    websocket.onopen = function (evt) {
        onOpen(evt)
    };
    websocket.onclose = function (evt) {
        onClose(evt)
    };
    websocket.onmessage = function (evt) {
        onMessage(evt)
    };
    websocket.onerror = function (evt) {
        onError(evt)
    };
}

function onOpen(evt) {
    writeToScreen("CONNECTED");
    var Response = builder.build("connection.ConnectionResponse");
    var response = new Response();
    response.connectionId = 42;
    doSend(response.toBuffer());
    console.log(response);
}

function onClose(evt) {
    writeToScreen("DISCONNECTED");
}

function onMessage(evt) {
    writeToScreen('<span style="color: blue;">RESPONSE: ' + evt.data + '</span>');
    websocket.close();
}

function onError(evt) {
    writeToScreen('<span style="color: red;">ERROR:</span> ' + evt.data);
}

function doSend(message) {
    writeToScreen("SENT: " + message);
    websocket.send(message);
}

function writeToScreen(message) {
    var pre = document.createElement("p");
    pre.style.wordWrap = "break-word";
    pre.innerHTML = message;
    output.appendChild(pre);
}
