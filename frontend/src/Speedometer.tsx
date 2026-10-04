const MAX=1000;
const fraction=(speed:number)=>Math.log1p(Math.max(0,Math.min(MAX,speed)))/Math.log1p(MAX);
const point=(angle:number,radius:number)=>{const rad=angle*Math.PI/180;return {x:240+radius*Math.cos(rad),y:210+radius*Math.sin(rad)}};
const angle=(speed:number)=>150+240*fraction(speed);
const arc=(()=>{const start=point(150,160),end=point(390,160);return `M${start.x},${start.y} A160,160 0 1 1 ${end.x},${end.y}`})();
type Props={value:number;active:boolean;upload:boolean};
export function Speedometer({value,active,upload}:Props){
 const ticks=[0,1,5,10,50,100,500,1000];
 const needle=point(angle(active?value:0),143);
 return <svg className={'speedometer '+(upload?'upload-dial':'')} viewBox="0 0 480 370" role="img" aria-label="实时速度表" data-speed={value.toFixed(3)}>
  <path className="gauge-track" d={arc} pathLength="100"/>
  <path className="gauge-fill" d={arc} pathLength="100" strokeDasharray={`${active?fraction(value)*100:0} 100`}/>
  {Array.from({length:49},(_,i)=>{const outer=point(150+i*5,148),inner=point(150+i*5,i%4===0?136:142);return <line key={i} className="gauge-tick" x1={inner.x} y1={inner.y} x2={outer.x} y2={outer.y}/>})}
  {ticks.map(speed=>{const pos=point(angle(speed),190);return <text key={speed} className="gauge-label" x={pos.x} y={pos.y}>{speed===1000?'1k+':speed}</text>})}
  {active&&<g className="gauge-needle"><line x1="240" y1="210" x2={needle.x} y2={needle.y}/><circle cx="240" cy="210" r="5"/></g>}
 </svg>
}
