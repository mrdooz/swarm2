var UTILS = (function(){

	var utils = {};

	utils.fnv32a = function(str) {
	    var FNV1_32A_INIT = 0x811c9dc5;
	    var hval = FNV1_32A_INIT;
	    for (var i = 0; i < str.length; ++i)
	    {
	        hval ^= str.charCodeAt(i);
	        hval += (hval << 1) + (hval << 4) + (hval << 7) + (hval << 8) + (hval << 24);
	    }
	    return hval >>> 0;
	};

	utils.concatBuffers = function(bufs) {
	    var len = 0;
	    _.each(bufs, function(e, i, l) { len += e.length; });

	    var tmp = new Uint8Array(len);
	    var ofs = 0;
	    _.each(bufs, function(e, i, l) {
	        tmp.set(new Uint8Array(e), ofs);
	        ofs += e.length;
	    });

	    return tmp.buffer;
	};

	return utils;

})();