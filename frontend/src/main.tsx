import React, {useEffect, useMemo, useState} from 'react';
import {createRoot} from 'react-dom/client';
import {FluentProvider,webLightTheme,webDarkTheme,Button,Card,Select,Input,Field,Table,TableHeader,TableRow,TableHeaderCell,TableBody,TableCell,Switch} from '@fluentui/react-components';
import {ArrowDown24Regular,ArrowUp24Regular,Play24Filled,Dismiss24Regular,Globe24Regular,History24Regular,Settings24Regular,Search24Regular,WeatherMoon24Regular,WeatherSunny24Regular} from '@fluentui/react-icons';
import './style.css';

type Server={hostid:string;hostip:string;hostname:string;location:string;oper:string;pname:string;port:string};
type Transfer={mbps:number;bytes:number;elapsedMs:number;budgetReached:boolean};
type Result={time:string;server:Server;path:string;tcpMedianMs:number;tcpJitterMs:number;httpMedianMs?:number;httpJitterMs?:number;download:Transfer;upload:Transfer;released:boolean};
type Progress={phase:string;mbps:number;bytes:number;elapsedMs:number;tcpMedianMs?:number;tcpJitterMs?:number;httpMedianMs?:number;httpJitterMs?:number};
type Options={serverId:string;durationSeconds:number;connections:number;maxMiB:number};
type Bridge={ListServers:()=>Promise<Server[]>;History:()=>Promise<Result[]>;StartSpeed:(o:Options)=>Promise<void>;StopSpeed:()=>Promise<void>};
declare global{interface Window{go?:{main:{App:Bridge}};runtime?:{EventsOn:(name:string,handler:(data:any)=>void,max:number)=>void;EventsOff:(name:string)=>void}}}
const bridge=()=>window.go?.main.App;
const number=(value:number|undefined)=>value===undefined?'—':value.toFixed(2);
const phaseNames:Record<string,string>={latency:'测量 HTTP 响应延迟',session:'申请节点会话',download:'正在测量下载',upload:'正在测量上传'};
function App(){
 const [dark,setDark]=useState(localStorage.getItem('theme')?localStorage.getItem('theme')==='dark':true);
 const [page,setPage]=useState('speed');const [servers,setServers]=useState<Server[]>([]);const [records,setRecords]=useState<Result[]>([]);
 const [province,setProvince]=useState('上海');const [operator,setOperator]=useState('');const [search,setSearch]=useState('');const [serverId,setServerId]=useState('');
 const [duration,setDuration]=useState(5);const [connections,setConnections]=useState(2);const [budget,setBudget]=useState(64);
 const [latency,setLatency]=useState<{median:number;jitter:number}|null>(null);
 const [running,setRunning]=useState(false);const [status,setStatus]=useState('准备就绪');const [error,setError]=useState('');
 const [progress,setProgress]=useState<Progress|null>(null);const [result,setResult]=useState<Result|null>(null);const [series,setSeries]=useState<Progress[]>([]);const [tablePage,setTablePage]=useState(0);
 const provinces=useMemo(()=>[...new Set(servers.map(s=>s.pname))],[servers]);
 const filtered=useMemo(()=>servers.filter(s=>(!province||s.pname===province)&&(!operator||s.oper===operator)&&(!search||`${s.hostname} ${s.hostip} ${s.location}`.includes(search))),[servers,province,operator,search]);
 const selected=servers.find(s=>s.hostid===serverId);
 useEffect(()=>{if(!filtered.some(s=>s.hostid===serverId))setServerId(filtered[0]?.hostid||'');setTablePage(0)},[filtered]);
 useEffect(()=>{setLatency(null);setResult(null);setProgress(null);setSeries([]);setError('');setStatus('准备就绪')},[serverId]);
 useEffect(()=>{document.documentElement.dataset.theme=dark?'dark':'light';localStorage.setItem('theme',dark?'dark':'light')},[dark]);
 useEffect(()=>{
  const app=bridge();if(!app){setStatus('请使用桌面程序启动测速');return}
  app.ListServers().then(setServers).catch(e=>setError(String(e)));app.History().then(setRecords).catch(e=>setError(String(e)));
  const events:{[name:string]:(data:any)=>void}={
   'speed:progress':(p:Progress)=>{if(p.httpMedianMs!==undefined&&p.httpJitterMs!==undefined)setLatency({median:p.httpMedianMs,jitter:p.httpJitterMs});setProgress(p);setStatus(phaseNames[p.phase]||p.phase);if(p.phase==='download'||p.phase==='upload')setSeries(old=>[...old,p].slice(-120))},
   'speed:result':(r:Result)=>{setResult(r);setRunning(false);setStatus('测速完成 · 会话已释放');app.History().then(setRecords).catch(e=>setError(String(e)))},
   'speed:error':(e:string)=>{setRunning(false);setProgress(null);setResult(null);setStatus(e.includes('context canceled')?'测速已停止':'测速失败');setError(e==='context canceled'?'':e)},
   'speed:notice':(e:string)=>setError(e),
  };
  Object.entries(events).forEach(([name,handler])=>window.runtime?.EventsOn(name,handler,-1));
  return()=>Object.keys(events).forEach(name=>window.runtime?.EventsOff(name));
 },[]);
 async function start(){const app=bridge();if(!app)return;setRunning(true);setLatency(null);setError('');setResult(null);setProgress(null);setSeries([]);setStatus('连接所选节点…');try{await app.StartSpeed({serverId,durationSeconds:duration,connections,maxMiB:budget})}catch(e){setRunning(false);setStatus('测速失败');setError(String(e))}}
 async function stop(){setStatus('正在停止并释放会话…');try{await bridge()?.StopSpeed()}catch(e){setError(String(e))}}
 const max=Math.max(1,...series.map(p=>p.mbps));
 const chartPath=(phase:string)=>series.map((p,i)=>({p,i})).filter(x=>x.p.phase===phase).map((x,i)=>`${i?'L':'M'}${x.i*700/Math.max(1,series.length-1)},${115-x.p.mbps/max*100}`).join(' ');
 const nodeFilters=<div className="filters"><Field label="省份"><Select value={province} disabled={running} onChange={(_,d)=>{setProvince(d.value);setOperator('')}}><option value="">全部省份</option>{provinces.map(p=><option key={p}>{p}</option>)}</Select></Field><Field label="运营商"><Select value={operator} disabled={running} onChange={(_,d)=>setOperator(d.value)}><option value="">全部运营商</option>{['电信','联通','移动','教育网','广电网'].map(o=><option key={o}>{o}</option>)}</Select></Field></div>;
 return <FluentProvider theme={dark?webDarkTheme:webLightTheme} className="app" style={{backgroundColor:'var(--bg)',color:'var(--fg)'}}>
  <main><header><button className="brand" onClick={()=>setPage('speed')}><Globe24Regular/><span>GlobalSpeed</span></button><nav>{[{id:'speed',name:'网络测速'},{id:'history',name:'历史记录'},{id:'settings',name:'设置'}].map(item=><Button key={item.id} appearance="transparent" className={page===item.id?'nav-selected':''} onClick={()=>setPage(item.id)}>{item.name}</Button>)}<Button appearance="transparent" icon={dark?<WeatherSunny24Regular/>:<WeatherMoon24Regular/>} aria-label="切换主题" onClick={()=>setDark(!dark)}/></nav></header>
  <div className={page==='speed'?'content speed-content':'content secondary-content'}>
  {page!=='speed'&&<div className="page-heading"><h1>{({nodes:'选择测速节点',history:'测速历史',settings:'设置'} as Record<string,string>)[page]}</h1><p>{page==='nodes'?'按地区与运营商筛选节点':page==='history'?'最近 100 次成功测量':'采样参数与界面外观'}</p></div>}
  {error&&<div className="error" role="alert">{error}</div>}
  {page==='speed'&&<>
   <div className="headline-metrics"><div className="headline-metric download"><span><ArrowDown24Regular/>下载 <small>Mbps</small></span><strong>{number(result?.download.mbps ?? (running?[...series].reverse().find(p=>p.phase==='download')?.mbps:undefined))}</strong></div><div className="headline-metric upload"><span><ArrowUp24Regular/>上传 <small>Mbps</small></span><strong>{number(result?.upload.mbps ?? (running?[...series].reverse().find(p=>p.phase==='upload')?.mbps:undefined))}</strong></div></div>
   <div className="latency-row"><span title="5 次请求发完到首个响应字节的中位数，包含节点处理时间；不是 ICMP Ping">HTTP 延迟 <b>{number(result?.httpMedianMs??latency?.median)}</b><small>ms</small></span><i/><span title="相邻 HTTP 延迟样本差值绝对值的平均值">抖动 <b>{number(result?.httpJitterMs??latency?.jitter)}</b><small>ms</small></span></div>
   <div className={'test-stage '+(running?'is-running':'')}>
    <svg className="dial" viewBox="0 0 320 320" aria-hidden="true"><circle className="dial-track" cx="160" cy="160" r="144"/><circle className="dial-accent" cx="160" cy="160" r="144"/></svg>
    {running?<div className="dial-reading"><span>{progress?.phase==='upload'?'上传':progress?.phase==='download'?'下载':'连接中'}</span><strong>{progress?.phase==='download'||progress?.phase==='upload'?number(progress.mbps):'—'}</strong><small>Mbps</small><Button appearance="transparent" icon={<Dismiss24Regular/>} onClick={stop}>停止测速</Button></div>:<button className="go-button" aria-label="开始测速" disabled={!selected||!bridge()} onClick={start}><span>{result?'再测一次':'开始'}</span><small>{result?'TEST AGAIN':'GO'}</small></button>}
   </div>
   <div className="status" role="status">{status}</div>
   {series.length>0&&<svg className="speed-chart" viewBox="0 0 700 130" role="img" aria-label="下载和上传吞吐曲线"><path className="grid-line" d="M0 115H700"/><path className="download-line" d={chartPath('download')}/><path className="upload-line" d={chartPath('upload')}/></svg>}
   <div className="connection-info"><div className="device-info"><Globe24Regular/><div><span>本机直连</span><strong>你的设备</strong></div></div><div className="server-info"><div><span>测速服务器</span><strong>{selected?.hostname||'选择节点'}</strong><small>{selected?`${selected.hostip}:${selected.port}`:'606 个运营商节点'}</small></div><Button appearance="transparent" disabled={running} onClick={()=>setPage('nodes')}>更换节点</Button></div></div>
   <div className="test-caption">{duration} 秒 / 阶段 <span>·</span> {connections} 个连接 <span>·</span> {budget} MiB 预算<Button appearance="transparent" size="small" disabled={running} onClick={()=>setPage('settings')}>调整</Button></div>
  </>}
  {page==='nodes'&&<Card>{nodeFilters}<Input contentBefore={<Search24Regular/>} placeholder="搜索名称、地址或城市" value={search} onChange={(_,d)=>setSearch(d.value)}/><Table><TableHeader><TableRow>{['节点','地区','运营商','地址',''].map(x=><TableHeaderCell key={x}>{x}</TableHeaderCell>)}</TableRow></TableHeader><TableBody>{filtered.slice(tablePage*30,(tablePage+1)*30).map(s=><TableRow key={s.hostid}><TableCell>{s.hostname}</TableCell><TableCell>{s.pname} · {s.location}</TableCell><TableCell>{s.oper||'其他'}</TableCell><TableCell className="mono">{s.hostip}:{s.port}</TableCell><TableCell><Button size="small" disabled={running} onClick={()=>{setServerId(s.hostid);setPage('speed')}}>选择</Button></TableCell></TableRow>)}</TableBody></Table><div className="pagination"><span>{filtered.length} 个节点 · 第 {tablePage+1} 页</span><Button disabled={tablePage===0} onClick={()=>setTablePage(tablePage-1)}>上一页</Button><Button disabled={(tablePage+1)*30>=filtered.length} onClick={()=>setTablePage(tablePage+1)}>下一页</Button></div></Card>}
  {page==='history'&&<Card>{records.length?<Table><TableHeader><TableRow>{['时间','节点','下载 Mbps','上传 Mbps','延迟 ms'].map(x=><TableHeaderCell key={x}>{x}</TableHeaderCell>)}</TableRow></TableHeader><TableBody>{records.map((r,i)=><TableRow key={i}><TableCell>{new Date(r.time).toLocaleString()}</TableCell><TableCell>{r.server.hostname}</TableCell><TableCell>{number(r.download.mbps)}</TableCell><TableCell>{number(r.upload.mbps)}</TableCell><TableCell>{number(r.httpMedianMs??r.tcpMedianMs)} {r.httpMedianMs===undefined?'TCP':'HTTP'}</TableCell></TableRow>)}</TableBody></Table>:<div className="empty"><History24Regular/><h2>还没有测速记录</h2><p>完成一次测速后，结果会出现在这里。</p></div>}</Card>}
  {page==='settings'&&<Card className="settings-card"><h2>采样参数</h2><Field label="每阶段时长"><Select disabled={running} value={duration} onChange={(_,d)=>setDuration(Number(d.value))}>{[5,10,15,30].map(n=><option key={n} value={n}>{n} 秒</option>)}</Select></Field><Field label="并发连接"><Select disabled={running} value={connections} onChange={(_,d)=>setConnections(Number(d.value))}>{[1,2,4,6].map(n=><option key={n} value={n}>{n}</option>)}</Select></Field><Field label="总流量预算"><Select disabled={running} value={budget} onChange={(_,d)=>setBudget(Number(d.value))}>{[64,128,256,512].map(n=><option key={n} value={n}>{n} MiB</option>)}</Select></Field><h2>外观</h2><Switch label="深色模式" checked={dark} onChange={(_,d)=>setDark(d.checked)}/><p className="muted">测速直接从设备连接所选运营商。所有历史记录均保存在本机。</p></Card>}
  </div></main>
 </FluentProvider>
}
document.documentElement.dataset.platform=navigator.userAgent.includes('Windows')?'windows':'other';
createRoot(document.getElementById('root')!).render(<App/>);
