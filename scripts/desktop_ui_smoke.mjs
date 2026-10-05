// Exercises desktop UI bindings with mock Go events; it does not emulate a
// native OS window or produce a real network speed result.
import {chromium} from '../reverse/.tools/browser/node_modules/playwright/index.mjs';
import fs from 'node:fs/promises';
import path from 'node:path';
import {execFileSync} from 'node:child_process';
import assert from 'node:assert/strict';
const servers=JSON.parse(execFileSync('go',['run','-buildvcs=false','./cmd/globalspeed','nodes','--no-update','--json'],{encoding:'utf8',env:{...process.env,GOCACHE:path.resolve('.buildcache/go'),GOMODCACHE:path.resolve('.buildcache/mod')}}));
const browser=await chromium.launch({headless:true,executablePath:'/home/abler/.cache/ms-playwright/chromium_headless_shell-1243/chrome-headless-shell-linux64/chrome-headless-shell',args:['--no-sandbox'],env:{...process.env,LD_LIBRARY_PATH:path.resolve('reverse/.tools/browser-libs/usr/lib/x86_64-linux-gnu')}});
try{
 const page=await browser.newPage({viewport:{width:520,height:720}});const errors=[];page.on('pageerror',e=>errors.push(e.message));
 await page.addInitScript(({servers})=>{
  const listeners={};let timer,records=[];let n=0;
  const emit=(name,data)=>listeners[name]?.(data);
  window.__emit=emit;window.__pauseMock=()=>clearInterval(timer);
  window.runtime={EventsOn:(name,handler)=>{listeners[name]=handler},EventsOff:name=>delete listeners[name]};
  window.go={main:{App:{NetworkInfo:async()=>({ip:'203.0.113.9',operator:'电信',province:'江苏',city:'南京',location:'中国 · 江苏 · 南京'}),SelectServer:async options=>{window.__matchOptions=options;window.__matchCount=(window.__matchCount||0)+1;return servers.find(s=>s.hostid==='1503')},ListServers:async()=>servers,History:async()=>records,StartSpeed:async options=>{
   window.__options=options;window.__matchesAtStart=window.__matchCount;emit('speed:progress',{phase:'session',mbps:0,bytes:0,elapsedMs:0,ping:{method:'icmp',averageMs:29.28,jitterMs:1.4,status:0,sent:10,received:10,packetLossPct:0}});n=0;clearInterval(timer);timer=setInterval(()=>{
    n++;emit('speed:progress',{phase:n<5?'download':'upload',mbps:n*12,bytes:n*1024,elapsedMs:n*80});
    if(n===10){clearInterval(timer);const r={time:new Date().toISOString(),server:servers.find(s=>s.hostid===options.serverId),path:'device-to-carrier',tcpMedianMs:.5,tcpJitterMs:.1,ping:{method:'icmp',averageMs:29.28,jitterMs:1.4,status:0,sent:10,received:10,packetLossPct:0},download:{mbps:92,bytes:1000000,elapsedMs:300,budgetReached:false},upload:{mbps:81,bytes:1000000,elapsedMs:400,budgetReached:false},released:true};records=[r,...records];emit('speed:result',r)}
   },80);
  },StopSpeed:async()=>{clearInterval(timer);emit('speed:error',{code:990,message:'用户中止测试'})}}}};
 },{servers});
 await page.goto('http://127.0.0.1:5173');
 await page.getByRole('button',{name:'开始测速',exact:true}).waitFor();
 await page.waitForFunction(()=>!document.querySelector('button[disabled]'));
 const checkFits=async()=>{
  const layout=await page.evaluate(()=>{const caption=document.querySelector('.test-caption').getBoundingClientRect();const stage=document.querySelector('.test-stage').getBoundingClientRect();const reading=document.querySelector('.dial-reading')?.getBoundingClientRect();const status=document.querySelector('.status').getBoundingClientRect();return {height:innerHeight,bottom:caption.bottom,scroll:document.documentElement.scrollHeight,width:innerWidth,right:caption.right,readingBottom:reading?.bottom,statusTop:status.top,stageBottom:stage.bottom}});
  assert.ok(layout.bottom<=layout.height,JSON.stringify(layout));assert.ok(layout.scroll<=layout.height,JSON.stringify(layout));assert.ok(layout.right<=layout.width,JSON.stringify(layout));if(layout.readingBottom)assert.ok(layout.readingBottom<=layout.statusTop,JSON.stringify(layout));
 };
 for(const viewport of [{width:480,height:640},{width:520,height:720},{width:800,height:520},{width:1280,height:720}]){await page.setViewportSize(viewport);await checkFits()}
 await page.setViewportSize({width:520,height:720});
 assert.equal(await page.locator('.server-info strong').innerText(),'南京电信');assert.ok((await page.locator('.public-ip').innerText()).includes('203.0.113.9'));
 const blocks=await page.evaluate(()=>({server:document.querySelector('.server-info').getBoundingClientRect().top,device:document.querySelector('.device-info').getBoundingClientRect().top}));assert.ok(blocks.device>blocks.server);assert.ok(await page.evaluate(()=>window.__matchCount>=1));
 await page.screenshot({path:'docs/desktop-preview.png',fullPage:true});
 await page.getByRole('button',{name:'更换节点',exact:true}).click();
 await page.getByRole('combobox',{name:'省份',exact:true}).selectOption('江苏');
 await page.getByRole('combobox',{name:'运营商',exact:true}).selectOption('电信');
 await page.getByRole('button',{name:'选择',exact:true}).first().click();
 await page.getByRole('button',{name:'开始测速',exact:true}).click();
 await page.locator('.latency-row').getByText('29.28',{exact:true}).waitFor();
 await page.getByText('测试完成 · 会话已释放',{exact:true}).waitFor();
 await page.waitForFunction(()=>document.querySelector('.speedometer').dataset.morph==='exit');
 await page.waitForFunction(()=>document.querySelector('.speed-content').classList.contains('result-state'));
 for(const viewport of [{width:480,height:640},{width:520,height:720},{width:800,height:520},{width:1280,height:720}]){await page.setViewportSize(viewport);await checkFits()}
 await page.setViewportSize({width:520,height:720});
 assert.equal(await page.locator('.headline-metric.download strong').innerText(),'92.00');assert.equal(await page.locator('.headline-metric.upload strong').innerText(),'81.00');
 const options=await page.evaluate(()=>window.__options);assert.equal(options.durationSeconds,5);assert.equal(options.connections,2);assert.ok(servers.some(s=>s.hostid===options.serverId&&s.pname==='江苏'&&s.oper==='电信'));
 await page.getByRole('button',{name:'历史记录',exact:true}).click();await page.getByRole('cell',{name:'92.00',exact:true}).waitFor();
 await page.getByRole('button',{name:'网络测速',exact:true}).click();await page.getByRole('button',{name:'更换节点',exact:true}).click();await page.getByPlaceholder('搜索名称、地址或城市').fill('南京');assert.ok(await page.getByRole('row').count()>1);
 await page.getByRole('button',{name:'网络测速',exact:true}).click();await page.getByRole('button',{name:'开始测速',exact:true}).click();await page.getByRole('button',{name:'停止测速',exact:true}).click();await page.locator('.status').getByText('用户中止测试',{exact:true}).waitFor();
 await page.getByRole('button',{name:'更换节点',exact:true}).click();await page.getByRole('button',{name:'自动选择节点',exact:true}).click();await page.getByRole('button',{name:'开始测速',exact:true}).click();await page.getByText('测试完成 · 会话已释放',{exact:true}).waitFor();assert.equal(await page.evaluate(()=>window.__options.auto),true);
 await page.getByRole('button',{name:'开始测速',exact:true}).click();
 await page.evaluate(()=>{window.__pauseMock();window.__emit('speed:progress',{phase:'download',mbps:0,bytes:1,elapsedMs:1})});
 await page.waitForFunction(()=>Number(document.querySelector('.speedometer').dataset.speed)===0);
 await page.evaluate(()=>window.__emit('speed:progress',{phase:'download',mbps:500,bytes:1000,elapsedMs:250}));
 await page.waitForFunction(()=>{const value=Number(document.querySelector('.speedometer').dataset.speed);return value>5&&value<450});
 const intermediate=await page.locator('.speedometer').getAttribute('data-speed');assert.ok(Number(intermediate)>0&&Number(intermediate)<500,'display must animate instead of jumping to raw sample');
 await page.waitForFunction(()=>Number(document.querySelector('.speedometer').dataset.speed)>490);
 const geometry=await page.evaluate(async()=>{const {gaugeAngle,curve}=await import('/src/gaugeMotion.ts');return {angles:[0,5,10,50,100,250,500,750,1000,2000].map(gaugeAngle),mid:gaugeAngle(25),ease:curve(.5)}});
 assert.deepEqual(geometry.angles,[-135,-101.25,-67.5,-33.75,0,33.75,67.5,101.25,135,135]);assert.equal(geometry.mid,-54.84375);assert.ok(geometry.ease>.68&&geometry.ease<.69);
 await page.waitForFunction(()=>!document.querySelector('.gauge-morph'));
 await page.waitForFunction(()=>[...document.querySelectorAll('.gauge-label')].every(e=>Number(getComputedStyle(e).opacity)>.99));
 assert.equal(await page.locator('.speed-chart').count(),0);
 assert.equal(await page.getByRole('img',{name:'下载和上传吞吐曲线'}).count(),0);
 for(const viewport of [{width:480,height:640},{width:520,height:720},{width:800,height:520},{width:1280,height:720}]){await page.setViewportSize(viewport);await checkFits()}
 await page.setViewportSize({width:520,height:720});
 await page.screenshot({path:'docs/desktop-measuring.png',fullPage:true});
 await page.evaluate(()=>window.__emit('speed:progress',{phase:'upload',mbps:0,bytes:0,elapsedMs:0}));
 await page.waitForFunction(()=>document.querySelector('.speedometer').dataset.switching==='true');
 assert.ok(Number(await page.locator('.speedometer').getAttribute('data-angle'))>-135,'phase transition should return instead of snapping');
 await page.waitForFunction(()=>Number(document.querySelector('.speedometer').dataset.speed)===0);
 assert.equal(await page.locator('.speedometer').getAttribute('data-angle'),'-135.000');
 await page.getByRole('button',{name:'停止测速',exact:true}).click();await page.locator('.status').getByText('用户中止测试',{exact:true}).waitFor();
 await page.getByRole('button',{name:'设置',exact:true}).click();const beforeTheme=await page.evaluate(()=>document.documentElement.dataset.theme);await page.getByRole('switch',{name:'深色模式'}).click();assert.notEqual(await page.evaluate(()=>document.documentElement.dataset.theme),beforeTheme);
 await page.getByRole('button',{name:'网络测速',exact:true}).click();
 await page.evaluate(()=>window.__emit('speed:error',{code:132,message:'测速服务器繁忙,请稍后重试'}));
 await page.locator('.status').getByText('测速服务器繁忙,请稍后重试',{exact:true}).waitFor();
 await page.evaluate(()=>window.__emit('speed:error',{code:121,message:'云服务器连接失败'}));
 await page.getByRole('dialog').waitFor();
 await page.getByRole('button',{name:'取消',exact:true}).click();
 await page.getByRole('dialog').waitFor({state:'hidden'});
 await page.evaluate(()=>window.__emit('speed:error',{code:122,message:'云服务器响应错误'}));
 await page.getByRole('button',{name:'重试',exact:true}).click();
 await page.locator('.status').getByText('准备就绪',{exact:true}).waitFor();await page.getByRole('button',{name:'开始测速',exact:true}).click();
 await page.getByText('测试完成 · 会话已释放',{exact:true}).waitFor();assert.equal(await page.evaluate(()=>window.__matchCount),await page.evaluate(()=>window.__matchesAtStart));
 assert.deepEqual(errors,[]);console.log('Desktop UI passed: Fluent rendering, node filters, Go options and events, history, search, stop, theme, speedometer smoothing, segmented scale, 200 ms easing, 500 ms return, entry/exit morph, no curve.');
}finally{await browser.close()}
