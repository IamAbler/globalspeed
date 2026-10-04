import React, {useEffect, useMemo, useState} from 'react';
import {createRoot} from 'react-dom/client';
import {FluentProvider,webLightTheme,webDarkTheme,Button,Card,Select,Input,Field,Table,TableHeader,TableRow,TableHeaderCell,TableBody,TableCell,Switch,Dialog,DialogSurface,DialogBody,DialogTitle,DialogContent,DialogActions} from '@fluentui/react-components';
import {ArrowDown24Regular,ArrowUp24Regular,Dismiss24Regular,Globe24Regular,Server24Regular,History24Regular,Settings24Regular,Search24Regular,WeatherMoon24Regular,WeatherSunny24Regular} from '@fluentui/react-icons';
import './style.css';
import {Speedometer} from './Speedometer';
import {useAnimatedSpeed} from './useAnimatedSpeed';

type Server={hostid:string;hostip:string;hostname:string;location:string;oper:string;pname:string;port:string};
type Transfer={mbps:number;bytes:number;elapsedMs:number;budgetReached:boolean};
type Ping={method:string;averageMs:number;jitterMs:number;sent:number;received:number;packetLossPct:number;status:number};
type Result={ping?:Ping;time:string;server:Server;path:string;tcpMedianMs:number;tcpJitterMs:number;httpMedianMs?:number;httpJitterMs?:number;download:Transfer;upload:Transfer;released:boolean};
type Progress={ping?:Ping;server?:Server;phase:string;mbps:number;bytes:number;elapsedMs:number;tcpMedianMs?:number;tcpJitterMs?:number;httpMedianMs?:number;httpJitterMs?:number};
type Options={auto:boolean;match:{province:string;operator:string};serverId:string;durationSeconds:number;connections:number;maxMiB:number};
type NetworkInfo={ip:string;operator:string;country:string;province:string;city:string;location:string};
type Bridge={NetworkInfo:()=>Promise<NetworkInfo>;SelectServer:(o:{ip:string;province:string;city:string;operator:string})=>Promise<Server>;ListServers:()=>Promise<Server[]>;History:()=>Promise<Result[]>;StartSpeed:(o:Options)=>Promise<void>;StopSpeed:()=>Promise<void>};
declare global{interface Window{go?:{main:{App:Bridge}};runtime?:{EventsOn:(name:string,handler:(data:any)=>void,max:number)=>void;EventsOff:(name:string)=>void}}}
const bridge=()=>window.go?.main.App;
const number=(value:number|undefined)=>value===undefined?'—':value.toFixed(2);
const phaseNames:Record<string,string>={latency:'测量 Ping',selecting:'正在自动选择节点',selected:'节点已选择',session:'申请节点会话',download:'正在测量下载',upload:'正在测量上传'};
function App(){
 const [dark,setDark]=useState(localStorage.getItem('theme')?localStorage.getItem('theme')==='dark':true);
 const [page,setPage]=useState('speed');const [servers,setServers]=useState<Server[]>([]);const [records,setRecords]=useState<Result[]>([]);
 const [province,setProvince]=useState('');const [operator,setOperator]=useState('');const [search,setSearch]=useState('');const [serverId,setServerId]=useState('');
 const [duration,setDuration]=useState(5);const [connections,setConnections]=useState(2);const [budget,setBudget]=useState(64);
 const [latency,setLatency]=useState<Ping|null>(null);
 const [auto,setAuto]=useState(true);const [autoNode,setAutoNode]=useState<Server|null>(null);
 const [networkInfo,setNetworkInfo]=useState<NetworkInfo|null>(null);const [networkLoading,setNetworkLoading]=useState(true);const [networkError,setNetworkError]=useState('');
 const [selecting,setSelecting]=useState(true);const [selectionVersion,setSelectionVersion]=useState(0);
 const [matchFailure,setMatchFailure]=useState(false);
 const [running,setRunning]=useState(false);const [closing,setClosing]=useState(false);const [status,setStatus]=useState('准备就绪');const [error,setError]=useState('');
 const [progress,setProgress]=useState<Progress|null>(null);const [result,setResult]=useState<Result|null>(null);const [rates,setRates]=useState<{download?:number;upload?:number}>({});const [tablePage,setTablePage]=useState(0);
 const provinces=useMemo(()=>[...new Set(servers.map(s=>s.pname))],[servers]);
 const filtered=useMemo(()=>servers.filter(s=>(!province||s.pname===province)&&(!operator||s.oper===operator)&&(!search||`${s.hostname} ${s.hostip} ${s.location}`.includes(search))),[servers,province,operator,search]);
 const selected=auto?autoNode:servers.find(s=>s.hostid===serverId);
 useEffect(()=>{if(!filtered.some(s=>s.hostid===serverId))setServerId(filtered[0]?.hostid||'');setTablePage(0)},[filtered]);
 useEffect(()=>{if(running)return;setLatency(null);setResult(null);setProgress(null);setRates({});setError('');if(!auto)setStatus('准备就绪')},[serverId]);
 useEffect(()=>{
  if(!auto){setSelecting(false);return}if(networkLoading)return;
  const app=bridge();if(!app){setSelecting(false);return}
  let active=true;setSelecting(true);setAutoNode(null);setResult(null);setLatency(null);setRates({});setClosing(false);setError('');setMatchFailure(false);setStatus('正在自动选择节点');
  app.SelectServer({ip:networkInfo?.ip||'',province:province||networkInfo?.province||'',city:province&&province!==networkInfo?.province?'':networkInfo?.city||'',operator:operator||networkInfo?.operator||''}).then(server=>{if(!active)return;setAutoNode(server);setStatus('准备就绪')}).catch(e=>{if(!active)return;const message=String(e);setStatus(`服务器匹配失败(${message}).`);setError(message);setMatchFailure(true)}).finally(()=>{if(active)setSelecting(false)});
  return()=>{active=false};
 },[auto,province,operator,networkLoading,networkInfo,selectionVersion]);
 useEffect(()=>{if(!error||matchFailure)return;const timer=setTimeout(()=>setError(''),4000);return()=>clearTimeout(timer)},[error,matchFailure]);
 useEffect(()=>{if(!closing)return;const timer=setTimeout(()=>setClosing(false),matchMedia('(prefers-reduced-motion: reduce)').matches?0:820);return()=>clearTimeout(timer)},[closing]);
 useEffect(()=>{document.documentElement.dataset.theme=dark?'dark':'light';localStorage.setItem('theme',dark?'dark':'light')},[dark]);
 useEffect(()=>{
  const app=bridge();if(!app){setStatus('请使用桌面程序启动测速');return}
  app.NetworkInfo().then(setNetworkInfo).catch(e=>setNetworkError(String(e))).finally(()=>setNetworkLoading(false));
  app.ListServers().then(setServers).catch(e=>setError(String(e)));app.History().then(setRecords).catch(e=>setError(String(e)));
  const events:{[name:string]:(data:any)=>void}={
   'speed:progress':(p:Progress)=>{if(p.ping)setLatency(p.ping);if(p.server)setAutoNode(p.server);setProgress(p);setStatus(phaseNames[p.phase]||p.phase);if(p.phase==='download'||p.phase==='upload')setRates(old=>({...old,[p.phase]:p.mbps}))},
   'speed:result':(r:Result)=>{setClosing(true);setResult(r);if(r.ping)setLatency(r.ping);setAutoNode(r.server);setRunning(false);setStatus(r.released?'测试完成 · 会话已释放':'测试完成');app.History().then(setRecords).catch(e=>setError(String(e)))},
   'speed:error':(e:{code:number;message:string}|string)=>{const code=typeof e==='string'?0:e.code;const message=typeof e==='string'?e:e.message;const matching=code>=121&&code<=123;setRunning(false);setClosing(false);setProgress(null);setResult(null);setRates({});setStatus(matching?`服务器匹配失败(${message}).`:message);setError(message);setMatchFailure(matching)},
   'speed:notice':(e:string)=>setError(e),
  };
  Object.entries(events).forEach(([name,handler])=>window.runtime?.EventsOn(name,handler,-1));
  return()=>Object.keys(events).forEach(name=>window.runtime?.EventsOff(name));
 },[]);
 async function start(){const app=bridge();if(!app)return;setMatchFailure(false);setClosing(false);setRunning(true);setLatency(null);setError('');setResult(null);setProgress(null);setRates({});setStatus('连接所选节点…');try{await app.StartSpeed({auto,match:{province,operator},serverId:selected?.hostid||serverId,durationSeconds:duration,connections,maxMiB:budget})}catch(e){setRunning(false);setStatus('测速失败');setError(String(e))}}
 async function stop(){setStatus('正在停止并释放会话…');try{await bridge()?.StopSpeed()}catch(e){setError(String(e))}}
 const shownPing=result?.ping??latency;
 const measuring=running&&(progress?.phase==='download'||progress?.phase==='upload');
 const motion=useAnimatedSpeed(progress?.mbps??0,measuring,progress?.phase??'idle');
 const liveSpeed=motion.value;
 const downSpeed=result?.download.mbps??(running?(progress?.phase==='download'?(motion.switching?undefined:liveSpeed):rates.download):undefined);
 const upSpeed=result?.upload.mbps??(running?(progress?.phase==='upload'?(motion.switching?undefined:liveSpeed):rates.upload):undefined);
 const nodeFilters=<div className="filters"><Field label="省份"><Select value={province} disabled={running} onChange={(_,d)=>{setProvince(d.value);setOperator('')}}><option value="">全部省份</option>{provinces.map(p=><option key={p}>{p}</option>)}</Select></Field><Field label="运营商"><Select value={operator} disabled={running} onChange={(_,d)=>setOperator(d.value)}><option value="">全部运营商</option>{['电信','联通','移动','教育网','广电网'].map(o=><option key={o}>{o}</option>)}</Select></Field></div>;
 return <FluentProvider theme={dark?webDarkTheme:webLightTheme} className="app" style={{backgroundColor:'var(--gs-bg)',color:'var(--gs-fg)'}}>
  <Dialog open={matchFailure} onOpenChange={(_,d)=>setMatchFailure(d.open)}><DialogSurface><DialogBody><DialogTitle>匹配服务器失败，是否重试?</DialogTitle><DialogContent>{error}</DialogContent><DialogActions><Button onClick={()=>setMatchFailure(false)}>取消</Button><Button appearance="primary" onClick={()=>{setMatchFailure(false);setSelectionVersion(v=>v+1)}}>重试</Button></DialogActions></DialogBody></DialogSurface></Dialog>
  <main className={`page-${page}`}><header><button className="brand" onClick={()=>setPage('speed')}><Globe24Regular/><span>GlobalSpeed</span></button><nav>{[{id:'speed',name:'网络测速'},{id:'history',name:'历史记录'},{id:'settings',name:'设置'}].map(item=><Button key={item.id} appearance="transparent" className={page===item.id?'nav-selected':''} onClick={()=>setPage(item.id)}>{item.name}</Button>)}<Button appearance="transparent" icon={dark?<WeatherSunny24Regular/>:<WeatherMoon24Regular/>} aria-label="切换主题" onClick={()=>setDark(!dark)}/></nav></header>
  <div className={page==='speed'?`content speed-content ${running||closing?'running-state':result?'result-state':'ready-state'}`:'content secondary-content'}>
  {page!=='speed'&&<div className="page-heading"><h1>{({nodes:'选择测速节点',history:'测速历史',settings:'设置'} as Record<string,string>)[page]}</h1><p>{page==='nodes'?'按地区与运营商筛选节点':page==='history'?'最近 100 次成功测量':'采样参数与界面外观'}</p></div>}
  {error&&<div className="error" role="alert">{error}</div>}
  {page==='speed'&&<>
   <div className="speed-utilities"><Button appearance="transparent" icon={<History24Regular/>} onClick={()=>setPage('history')}>历史记录</Button><Button appearance="transparent" icon={<Settings24Regular/>} onClick={()=>setPage('settings')}>设置</Button></div>
   <div className="headline-metrics"><div className="headline-metric download"><span><ArrowDown24Regular/>下载 <small>Mbps</small></span><strong>{number(downSpeed)}</strong></div><div className="headline-metric upload"><span><ArrowUp24Regular/>上传 <small>Mbps</small></span><strong>{number(upSpeed)}</strong></div></div>
   <div className="latency-row"><span title="优先 ICMP；不可用时按原版回退到 TCP，显示均值">Ping{shownPing?` · ${shownPing.method.toUpperCase()}`:''} <b>{number(shownPing?.status===0?shownPing.averageMs:undefined)}</b><small>ms</small></span><i/><span title="按原版算法统计相邻成功样本差值">抖动 <b>{number(shownPing?.status===0?shownPing.jitterMs:undefined)}</b><small>ms</small></span></div>
   <div className={'test-stage '+(running?'is-running':'')}>
    <Speedometer value={liveSpeed} angle={motion.angle} active={measuring} running={running} switching={motion.switching} upload={progress?.phase==='upload'}/>
    {running||closing?<div className="dial-reading"><span>{closing?'测速完成':motion.switching?'切换阶段':progress?.phase==='upload'?'上传':progress?.phase==='download'?'下载':'连接中'}</span><strong>{closing?number(result?.upload.mbps):progress?.phase==='download'||progress?.phase==='upload'?(motion.switching?'—':number(liveSpeed)):'—'}</strong><small>Mbps</small></div>:<button className="go-button" aria-label="开始测速" disabled={running||selecting||(auto&&networkLoading)||!selected||!bridge()} onClick={start}><span>GO</span><small>{result?'再测一次':'开始测速'}</small></button>}
   </div>
   <div className="status"><span role="status">{status}</span>{running&&<Button appearance="transparent" className="gauge-stop" icon={<Dismiss24Regular/>} onClick={stop}>停止测速</Button>}</div>
   <div className="connection-info">
    <div className="server-info"><Server24Regular/><div className="network-details"><span>测速节点{auto?' · 自动选择':''}</span><strong>{selected?.hostname||(selecting?'正在选择节点…':'未选择节点')}</strong><small>{selected?`${[selected.pname,selected.location].filter(Boolean).join(' · ')} · ${selected.hostip}:${selected.port}`:'请重试或手动选择节点'}</small></div><Button appearance="transparent" disabled={running} onClick={()=>setPage('nodes')}>更换节点</Button></div>
    <div className="device-info"><Globe24Regular/><div className="network-details"><span>本地网络 · 公网 IP</span><strong>{networkInfo?.operator||(networkLoading?'正在获取网络信息…':'运营商未知')}</strong><small className="public-ip">{networkInfo?.ip||'—'}{networkInfo?.location?` · ${networkInfo.location}`:''}</small>{networkError&&<small>网络信息获取失败</small>}</div><Button appearance="transparent" disabled={networkLoading||running} onClick={async()=>{setNetworkLoading(true);setNetworkError('');try{setNetworkInfo(await bridge()!.NetworkInfo())}catch(e){setNetworkError(String(e))}finally{setNetworkLoading(false)}}}>刷新</Button></div>
   </div>
   <div className="connection-mode"><span>连接模式</span><div><Button appearance="transparent" disabled={running} className={connections>1?'mode-active':''} onClick={()=>setConnections(2)}>多连接</Button><span className="connection-divider">↔</span><Button appearance="transparent" disabled={running} className={connections===1?'mode-active':''} onClick={()=>setConnections(1)}>单连接</Button></div></div>
   <div className="test-caption">{duration} 秒 / 阶段 <span>·</span> {connections} 个连接 <span>·</span> {budget} MiB 预算<Button appearance="transparent" size="small" disabled={running} onClick={()=>setPage('settings')}>调整</Button></div>
  </>}
  {page==='nodes'&&<Card><Button appearance="primary" disabled={running} onClick={()=>{setAuto(true);setSelectionVersion(v=>v+1);setAutoNode(null);setLatency(null);setResult(null);setRates({});setError('');setStatus('准备就绪');setPage('speed')}}>自动选择节点</Button>{nodeFilters}<Input contentBefore={<Search24Regular/>} placeholder="搜索名称、地址或城市" value={search} onChange={(_,d)=>setSearch(d.value)}/><Table><TableHeader><TableRow>{['节点','地区','运营商','地址',''].map(x=><TableHeaderCell key={x}>{x}</TableHeaderCell>)}</TableRow></TableHeader><TableBody>{filtered.slice(tablePage*30,(tablePage+1)*30).map(s=><TableRow key={s.hostid}><TableCell>{s.hostname}</TableCell><TableCell>{s.pname} · {s.location}</TableCell><TableCell>{s.oper||'其他'}</TableCell><TableCell className="mono">{s.hostip}:{s.port}</TableCell><TableCell><Button size="small" disabled={running} onClick={()=>{setAuto(false);setAutoNode(null);setLatency(null);setResult(null);setRates({});setError('');setStatus('准备就绪');setServerId(s.hostid);setPage('speed')}}>选择</Button></TableCell></TableRow>)}</TableBody></Table><div className="pagination"><span>{filtered.length} 个节点 · 第 {tablePage+1} 页</span><Button disabled={tablePage===0} onClick={()=>setTablePage(tablePage-1)}>上一页</Button><Button disabled={(tablePage+1)*30>=filtered.length} onClick={()=>setTablePage(tablePage+1)}>下一页</Button></div></Card>}
  {page==='history'&&<Card>{records.length?<Table><TableHeader><TableRow>{['时间','节点','下载 Mbps','上传 Mbps','延迟 ms'].map(x=><TableHeaderCell key={x}>{x}</TableHeaderCell>)}</TableRow></TableHeader><TableBody>{records.map((r,i)=><TableRow key={i}><TableCell>{new Date(r.time).toLocaleString()}</TableCell><TableCell>{r.server.hostname}</TableCell><TableCell>{number(r.download.mbps)}</TableCell><TableCell>{number(r.upload.mbps)}</TableCell><TableCell>{number(r.ping?(r.ping.status===0?r.ping.averageMs:undefined):r.httpMedianMs??r.tcpMedianMs)} {r.ping?r.ping.method.toUpperCase():r.httpMedianMs===undefined?'TCP':'HTTP'}</TableCell></TableRow>)}</TableBody></Table>:<div className="empty"><History24Regular/><h2>还没有测速记录</h2><p>完成一次测速后，结果会出现在这里。</p></div>}</Card>}
  {page==='settings'&&<Card className="settings-card"><h2>采样参数</h2><Field label="每阶段时长"><Select disabled={running} value={duration} onChange={(_,d)=>setDuration(Number(d.value))}>{[5,10,15,30].map(n=><option key={n} value={n}>{n} 秒</option>)}</Select></Field><Field label="并发连接"><Select disabled={running} value={connections} onChange={(_,d)=>setConnections(Number(d.value))}>{[1,2,4,6].map(n=><option key={n} value={n}>{n}</option>)}</Select></Field><Field label="总流量预算"><Select disabled={running} value={budget} onChange={(_,d)=>setBudget(Number(d.value))}>{[64,128,256,512].map(n=><option key={n} value={n}>{n} MiB</option>)}</Select></Field><h2>外观</h2><Switch label="深色模式" checked={dark} onChange={(_,d)=>setDark(d.checked)}/><p className="muted">测速直接从设备连接所选运营商。所有历史记录均保存在本机。</p></Card>}
  </div></main>
 </FluentProvider>
}
document.documentElement.dataset.platform=navigator.userAgent.includes('Windows')?'windows':'other';
createRoot(document.getElementById('root')!).render(<App/>);
