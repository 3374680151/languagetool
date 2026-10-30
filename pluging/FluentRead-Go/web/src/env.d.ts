/**
 * env.d.ts 声明 Vue 单文件组件与环境类型。
 *
 * @file web/src/env.d.ts
 * 文件职责：让 vue-tsc 识别 .vue 模块；声明 ImportMetaEnv。
 *
 * 模块边界：仅类型声明。
 */

/// <reference types="vite/client" />

declare module '*.vue' {
  import type { DefineComponent } from 'vue';
  const component: DefineComponent<object, object, unknown>;
  export default component;
}
