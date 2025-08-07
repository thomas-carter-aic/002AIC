"use strict";
Object.defineProperty(exports, "__esModule", { value: true });
exports.HotModuleServerPlugin = void 0;
class HotModuleServerPlugin {
    constructor(hmrPort) {
        this.hmrPort = hmrPort;
    }
    apply(compiler) {
        const express = require('express');
        const app = express();
        app.use(require('webpack-hot-middleware')(compiler));
        app.listen(this.hmrPort, () => { });
    }
}
exports.HotModuleServerPlugin = HotModuleServerPlugin;
//# sourceMappingURL=HotModuleServerPlugin.js.map