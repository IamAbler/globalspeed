import {defineConfig} from 'vite';
export default defineConfig({base:'./',build:{rollupOptions:{onwarn(warning,warn){
  // Fluent's React Server Component markers are redundant in this client bundle.
  if(warning.code==='MODULE_LEVEL_DIRECTIVE'&&warning.message.includes('use client'))return;
  warn(warning);
}}}});
