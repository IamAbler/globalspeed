import {useEffect,useId,useRef,useState} from 'react';
import {gaugeStart,gaugeTicks,curve} from './gaugeMotion';
const center={x:240,y:190},radius=160;
const point=(angle:number,r:number)=>{const rad=(angle-90)*Math.PI/180;return {x:center.x+r*Math.cos(rad),y:center.y+r*Math.sin(rad)}};
const start=point(-135,radius),end=point(135,radius);
const arc=`M${start.x},${start.y} A${radius},${radius} 0 1 1 ${end.x},${end.y}`;
type Props={value:number;angle:number;active:boolean;running:boolean;upload:boolean;switching:boolean};
export function Speedometer({value,angle,active,running,upload,switching}:Props){
 const id=useId().replace(/:/g,'');
 const lastAngle=useRef(gaugeStart);if(active)lastAngle.current=angle;
 const [morph,setMorph]=useState({mode:'idle',progress:0});
 const wasRunning=useRef(false);
 useEffect(()=>{
  const mode=running?'entry':wasRunning.current?'exit':'idle';wasRunning.current=running;
  if(mode==='idle'){setMorph({mode,progress:0});return;}
  if(matchMedia('(prefers-reduced-motion: reduce)').matches){setMorph({mode:running?'live':'idle',progress:running?1:0});return;}
  let frame=0;const started=performance.now(),duration=mode==='entry'?350:820;
  setMorph({mode,progress:0});
  const animate=(now:number)=>{const progress=Math.min(1,(now-started)/duration);setMorph({mode:progress===1?(running?'live':'idle'):mode,progress});if(progress<1)frame=requestAnimationFrame(animate)};
  frame=requestAnimationFrame(animate);return()=>cancelAnimationFrame(frame);
 },[running]);
 const entry=morph.mode==='entry'?morph.progress:morph.mode==='live'?1:0;
 const exiting=morph.mode==='exit',exitTime=morph.progress*820;
 const expanded=curve(Math.min(1,entry*350/200),true);
 const opened=curve(Math.max(0,(entry*350-200)/150),true);
 const displayAngle=active?angle:exiting?lastAngle.current:gaugeStart;
 const progress=(displayAngle-gaugeStart)/270;
 return <svg className={`speedometer ${upload?'upload-dial':''} ${switching?'gauge-switching':''}`} viewBox="0 0 480 370" role="img" aria-label="实时速度表" data-speed={value.toFixed(3)} data-angle={displayAngle.toFixed(3)} data-switching={switching} data-morph={morph.mode}>
  <defs>
   <linearGradient id={`${id}-arc`} x1="0" y1="1" x2="1" y2="0"><stop offset="0" stopColor="var(--gs-gauge-start)"/><stop offset="1" stopColor="var(--gs-gauge-end)"/></linearGradient>
   <linearGradient id={`${id}-needle`} x1="0" y1="0" x2="0" y2="1"><stop offset="0" stopColor="var(--gs-fg)" stopOpacity=".7"/><stop offset="1" stopColor="var(--gs-fg)" stopOpacity="0"/></linearGradient>
  </defs>
  {(morph.mode==='entry'||exiting)&&<circle className="gauge-morph" cx={center.x} cy={center.y}
   r={exiting?160-105*curve(Math.max(0,(exitTime-420)/220),true):55+105*expanded}
   fill="none" strokeWidth={exiting?28+82*curve(Math.max(0,(exitTime-420)/220),true):110-82*expanded}
   opacity={exiting?(exitTime<200?0:1-curve(Math.max(0,(exitTime-640)/180))):1}
   pathLength="100" strokeDasharray={`${exiting?75+25*curve(Math.max(0,(exitTime-200)/220),true):100-25*opened} 100`}
   transform={`rotate(135 ${center.x} ${center.y})`}/>}
  <g opacity={exiting?1-curve(Math.min(1,exitTime/100)):entry===1?1:0}>
   <path className="gauge-track" d={arc} pathLength="100"/>
   <path className="gauge-fill" d={arc} pathLength="100" stroke={`url(#${id}-arc)`} strokeDasharray={`${Math.max(0,progress)*100} 100`} opacity={(active||exiting)&&!switching?1:0}/>
   {gaugeTicks.map((speed,index)=>{const theta=gaugeStart+index*270/(gaugeTicks.length-1),pos=point(theta,121);return <text key={speed} className={`gauge-label ${displayAngle>=theta?'gauge-label-passed':''}`} x={pos.x} y={pos.y} style={{animationDelay:`${index*80}ms`}}>{speed===1000?'1000+':speed}</text>})}
   {(active||exiting)&&<g transform={`rotate(${displayAngle} ${center.x} ${center.y})`}><path className="gauge-needle-shape" d={`M230,190 L250,190 L243,80 L237,80 Z`} fill={`url(#${id}-needle)`}/></g>}
  </g>
 </svg>
}
