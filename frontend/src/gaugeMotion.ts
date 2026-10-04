// A segmented scale: each interval occupies the same angle, while values
// interpolate linearly inside that interval. Angles are measured from 12 o'clock.
export const gaugeTicks=[0,5,10,50,100,250,500,750,1000];
export const gaugeStart=-135;
export function gaugeAngle(speed:number){
 const value=Number.isFinite(speed)?Math.max(0,speed):0;
 const segment=gaugeTicks.findIndex(limit=>limit>=value);
 if(segment<0)return 135;
 if(segment===0)return gaugeStart;
 const lower=gaugeTicks[segment-1],upper=gaugeTicks[segment];
 return gaugeStart+((segment-1)+(value-lower)/(upper-lower))*270/(gaugeTicks.length-1);
}
export function curve(progress:number,inOut=false){
 const t=Math.min(1,Math.max(0,progress));if(t===0||t===1)return t;
 const x1=inOut?.42:0,x2=.58;
 const axis=(p:number,a:number,b:number)=>3*(1-p)*(1-p)*p*a+3*(1-p)*p*p*b+p*p*p;
 let lower=0,upper=1;
 for(let i=0;i<16;i++){const middle=(lower+upper)/2;if(axis(middle,x1,x2)<t)lower=middle;else upper=middle;}
 return axis((lower+upper)/2,0,1);
}
