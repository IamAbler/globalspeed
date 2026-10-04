import {useEffect,useRef,useState} from 'react';

// Presentation only: original samples and final results are not modified.
export function useAnimatedSpeed(target:number,active:boolean,phase:string){
 const [value,setValue]=useState(0);
 const current=useRef(0);
 const previousPhase=useRef(phase);
 useEffect(()=>{
  if(previousPhase.current!==phase || !active){current.current=0;setValue(0);previousPhase.current=phase;}
  if(!active)return;
  const reduced=matchMedia('(prefers-reduced-motion: reduce)').matches;
  const goal=Number.isFinite(target)?Math.max(0,target):0;
  if(reduced){current.current=goal;setValue(goal);return;}
  let frame=0,last=performance.now();
  const animate=(now:number)=>{
   const elapsed=Math.min(64,now-last);last=now;
   current.current+=(goal-current.current)*(1-Math.exp(-elapsed/220));
   if(Math.abs(goal-current.current)<.005){current.current=goal;setValue(goal);return;}
   setValue(current.current);frame=requestAnimationFrame(animate);
  };
  frame=requestAnimationFrame(animate);
  return()=>cancelAnimationFrame(frame);
 },[target,active,phase]);
 return value;
}
