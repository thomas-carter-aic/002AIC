import { Plugin, Compiler } from 'webpack';
export declare class HotModuleServerPlugin implements Plugin {
    private hmrPort;
    constructor(hmrPort: number);
    apply(compiler: Compiler): void;
}
