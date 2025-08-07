import * as webpack from 'webpack';
import type { LogLevels, BundleHandlerResponse } from 'piral-cli';
export declare function runWebpack(wpConfig: webpack.Configuration, logLevel: LogLevels): BundleHandlerResponse;
