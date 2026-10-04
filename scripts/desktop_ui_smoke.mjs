// Exercises desktop UI bindings with mock Go events; it does not emulate a
// native OS window or produce a real network speed result.
import {chromium} from '../reverse/.tools/browser/node_modules/playwright/index.mjs';
import fs from 'node:fs/promises';
import path from 'node:path';
import assert from 'node:assert/strict';
const servers=JSON.parse(await fs.readFile('internal/catalog/servers.json','utf8'));
const browser=await chromium.launch({headless:true,executablePath:'/home/abler/.cache/ms-playwright/chromium_headless_shell-1243/chrome-headless-shell-linux64/chrome-headless-shell',args:['--no-sandbox'],env:{...process.env,LD_LIBRARY_PATH:path.resolve('reverse/.tools/browser-libs/usr/lib/x86_64-linux-gnu')}});
try{
 const page=await browser.newPage({viewport:{width:960,height:640}});const errors=[];page.on('pageerror',e=>errors.push(e.message));
 await page.addInitScript(({servers})=>{
  const listeners={};let timer,records=[];let n=0;
  const emit=(name,data)=>listeners[name]?.(data);
  window.__emit=emit;window.__pauseMock=()=>clearInterval(timer);
  window.runtime={EventsOn:(name,handler)=>{listeners[name]=handler},EventsOff:name=>delete listeners[name]};
  window.go={main:{App:{ListServers:async()=>servers,History:async()=>records,StartSpeed:async options=>{
   window.__options=options;if(options.auto){options.serverId='1503';emit('speed:progress',{phase:'selected',server:servers.find(s=>s.hostid==='1503')})};emit('speed:progress',{phase:'session',mbps:0,bytes:0,elapsedMs:0,ping:{method:'icmp',averageMs:29.28,jitterMs:1.4,status:0,sent:10,received:10,packetLossPct:0}});n=0;clearInterval(timer);timer=setInterval(()=>{
    n++;emit('speed:progress',{phase:n<5?'download':'upload',mbps:n*12,bytes:n*1024,elapsedMs:n*80});
    if(n===10){clearInterval(timer);const r={time:new Date().toISOString(),server:servers.find(s=>s.hostid===options.serverId),path:'device-to-carrier',tcpMedianMs:.5,tcpJitterMs:.1,ping:{method:'icmp',averageMs:29.28,jitterMs:1.4,status:0,sent:10,received:10,packetLossPct:0},download:{mbps:92,bytes:1000000,elapsedMs:300,budgetReached:false},upload:{mbps:81,bytes:1000000,elapsedMs:400,budgetReached:false},released:true};records=[r,...records];emit('speed:result',r)}
   },80);
  },StopSpeed:async()=>{clearInterval(timer);emit('speed:error','context canceled')}}}};
 },{servers});
 await page.goto('http://127.0.0.1:5173');
 await page.getByRole('button',{name:'开始测速',exact:true}).waitFor();
 await page.waitForFunction(()=>!document.querySelector('button[disabled]'));
 const checkFits=async()=>{
  const layout=await page.evaluate(()=>{const caption=document.querySelector('.test-caption').getBoundingClientRect();const stage=document.querySelector('.test-stage').getBoundingClientRect();const reading=document.querySelector('.dial-reading')?.getBoundingClientRect();const status=document.querySelector('.status').getBoundingClientRect();return {height:innerHeight,bottom:caption.bottom,scroll:document.documentElement.scrollHeight,width:innerWidth,right:caption.right,readingBottom:reading?.bottom,statusTop:status.top,stageBottom:stage.bottom}});
  assert.ok(layout.bottom<=layout.height,JSON.stringify(layout));assert.ok(layout.scroll<=layout.height,JSON.stringify(layout));assert.ok(layout.right<=layout.width,JSON.stringify(layout));if(layout.readingBottom)assert.ok(layout.readingBottom<=layout.statusTop,JSON.stringify(layout));
 };
 for(const viewport of [{width:800,height:520},{width:960,height:640},{width:1280,height:720}]){await page.setViewportSize(viewport);await checkFits()}
 await page.setViewportSize({width:960,height:640});
 await page.screenshot({path:'docs/desktop-preview.png',fullPage:true});
 await page.getByRole('button',{name:'更换节点',exact:true}).click();
 await page.getByRole('combobox',{name:'省份',exact:true}).selectOption('江苏');
 await page.getByRole('combobox',{name:'运营商',exact:true}).selectOption('电信');
 await page.getByRole('button',{name:'选择',exact:true}).first().click();
 await page.getByRole('button',{name:'开始测速',exact:true}).click();
 await page.locator('.latency-row').getByText('29.28',{exact:true}).waitFor();
 await page.getByText('测速完成 · 会话已释放',{exact:true}).waitFor();
 for(const viewport of [{width:800,height:520},{width:960,height:640},{width:1280,height:720}]){await page.setViewportSize(viewport);await checkFits()}
 await page.setViewportSize({width:960,height:640});
 assert.equal(await page.locator('.headline-metric.download strong').innerText(),'92.00');assert.equal(await page.locator('.headline-metric.upload strong').innerText(),'81.00');
 const options=await page.evaluate(()=>window.__options);assert.equal(options.durationSeconds,5);assert.equal(options.connections,2);assert.ok(servers.some(s=>s.hostid===options.serverId&&s.pname==='江苏'&&s.oper==='电信'));
 await page.getByRole('button',{name:'历史记录',exact:true}).click();await page.getByRole('cell',{name:'92.00',exact:true}).waitFor();
 await page.getByRole('button',{name:'网络测速',exact:true}).click();await page.getByRole('button',{name:'更换节点',exact:true}).click();await page.getByPlaceholder('搜索名称、地址或城市').fill('南京');assert.ok(await page.getByRole('row').count()>1);
 await page.getByRole('button',{name:'网络测速',exact:true}).click();await page.getByRole('button',{name:'开始测速',exact:true}).click();await page.getByRole('button',{name:'停止测速',exact:true}).click();await page.getByText('测速已停止',{exact:true}).waitFor();
 await page.getByRole('button',{name:'更换节点',exact:true}).click();await page.getByRole('button',{name:'自动选择节点',exact:true}).click();await page.getByRole('button',{name:'开始测速',exact:true}).click();await page.getByText('测速完成 · 会话已释放',{exact:true}).waitFor();assert.equal(await page.evaluate(()=>window.__options.auto),true);
 await page.getByRole('button',{name:'开始测速',exact:true}).click();
 await page.evaluate(()=>{window.__pauseMock();window.__emit('speed:progress',{phase:'download',mbps:0,bytes:1,elapsedMs:1})});
 await page.waitForFunction(()=>Number(document.querySelector('.speedometer').dataset.speed)===0);
 await page.evaluate(()=>window.__emit('speed:progress',{phase:'download',mbps:500,bytes:1000,elapsedMs:250}));
 await page.waitForFunction(()=>{const value=Number(document.querySelector('.speedometer').dataset.speed);return value>5&&value<450});
 const intermediate=await page.locator('.speedometer').getAttribute('data-speed');assert.ok(Number(intermediate)>0&&Number(intermediate)<500,'display must animate instead of jumping to raw sample');
 await page.waitForFunction(()=>Number(document.querySelector('.speedometer').dataset.speed)>490);
 assert.equal(await page.locator('.speed-chart').count(),0);
 assert.equal(await page.getByRole('img',{name:'下载和上传吞吐曲线'}).count(),0);
 for(const viewport of [{width:800,height:520},{width:960,height:640},{width:1280,height:720}]){await page.setViewportSize(viewport);await checkFits()}
 await page.setViewportSize({width:960,height:640});
 await page.screenshot({path:'docs/desktop-measuring.png',fullPage:true});
 await page.evaluate(()=>window.__emit('speed:progress',{phase:'upload',mbps:0,bytes:0,elapsedMs:0}));
 await page.waitForFunction(()=>Number(document.querySelector('.speedometer').dataset.speed)===0);
 await page.getByRole('button',{name:'停止测速',exact:true}).click();await page.getByText('测速已停止',{exact:true}).waitFor();
 await page.getByRole('button',{name:'设置',exact:true}).click();const beforeTheme=await page.evaluate(()=>document.documentElement.dataset.theme);await page.getByRole('switch',{name:'深色模式'}).click();assert.notEqual(await page.evaluate(()=>document.documentElement.dataset.theme),beforeTheme);
 assert.deepEqual(errors,[]);console.log('Desktop UI passed: Fluent rendering, node filters, Go options and events, history, search, stop, theme, speedometer smoothing, phase reset, no curve.');
}finally{await browser.close()}
