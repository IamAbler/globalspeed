import {useEffect,useRef,useState} from 'react';
import {curve,gaugeAngle,gaugeStart} from './gaugeMotion';

type Motion={value:number;angle:number;switching:boolean};
const empty:Motion={value:0,angle:gaugeStart,switching:false};
// Presentation state only. Retarget from the currently displayed frame,
// with a dedicated 500 ms return before tracking the next direction.
export function useAnimatedSpeed(target:number,active:boolean,phase:string){
 const [motion,setMotion]=useState<Motion>(empty);
 const current=useRef<Motion>(empty);
 const previousPhase=useRef('idle');
 const returning=useRef<{start:number;from:Motion}|null>(null);
 useEffect(()=>{
  if(!active){current.current=empty;setMotion(empty);previousPhase.current='idle';returning.current=null;return;}
  const reduced=matchMedia('(prefers-reduced-motion: reduce)').matches;
  const goal=Number.isFinite(target)?Math.max(0,target):0;
  if(previousPhase.current!==phase){
   if(previousPhase.current!=='idle'&&!reduced)returning.current={start:performance.now(),from:current.current};
   previousPhase.current=phase;
  }
  if(reduced){current.current={value:goal,angle:gaugeAngle(goal),switching:false};setMotion(current.current);return;}
  let frame=0,started=performance.now(),from=current.current;
  const animate=(now:number)=>{
   if(returning.current){
    const reset=returning.current,progress=Math.min(1,(now-reset.start)/500),weight=curve(progress,true);
    current.current={value:reset.from.value*(1-weight),angle:reset.from.angle+(gaugeStart-reset.from.angle)*weight,switching:progress<1};
    if(progress===1){returning.current=null;started=now;from=current.current;}
   }else{
    const progress=Math.min(1,(now-started)/200),weight=curve(progress);
    current.current={value:from.value+(goal-from.value)*weight,angle:from.angle+(gaugeAngle(goal)-from.angle)*weight,switching:false};
    setMotion(current.current);
    if(progress===1)return;
   }
   setMotion(current.current);frame=requestAnimationFrame(animate);
  };
  frame=requestAnimationFrame(animate);
  return()=>cancelAnimationFrame(frame);
 },[target,active,phase]);
 return motion;
}
