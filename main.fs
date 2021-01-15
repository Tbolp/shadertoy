// #version 330 core

float alternate(float p, float d){;
	return sign(fract(p*d*.5)*2.-1.);
}

vec3 rainbow(float t){
  return sin(t+vec3(0,.33,.66)*6.28)*.5+.5;
}

vec3 TwinDragon(vec2 p){
  float time = fract(iTime*0.05)*20.;
  
  //scaling
  p = (p*2.-iResolution.xy)/iResolution.y*1.5;
  
  //----------the fractal stuff----   ---THIS IS ANIMATIONS----(so remove them if you want)
  p.y += alternate(p.x, 256. )/512. * clamp(time-16.,0.,2.)/2.;
  p.x -= alternate(p.y, 128. )/256. * clamp(time-14.,0.,2.)/2.;
  p.y += alternate(p.x,  64. )/128. * clamp(time-12.,0.,2.)/2.;
  p.x -= alternate(p.y,  32. )/ 64. * clamp(time-10.,0.,2.)/2.;
  p.y += alternate(p.x,  16. )/ 32. * clamp(time- 8.,0.,2.)/2.;
  p.x -= alternate(p.y,   8. )/ 16. * clamp(time- 6.,0.,2.)/2.;
  p.y += alternate(p.x,   4. )/  8. * clamp(time- 4.,0.,2.)/2.;
  p.x -= alternate(p.y,   2. )/  4. * clamp(time- 2.,0.,2.)/2.;

  // prettifying
  vec2  block  = ceil(p+.5);               //index for blocks from which the fractal is shifted
  vec3  color  = rainbow(block.x*4.+block.y);  //rainbow palette using block index as t
  float dis    = length(fract(p+.5)*2.-1.);//distance to middle of block
        // color *= .5+dis*.7;                    //using distance within block for some more pretty.
  
  return color;
}

void mainImage( out vec4 fragColor, in vec2 fragCoord ){
  vec2 d = vec2(.5,0);
  
  //some antialiasing
  vec3 col = (
      TwinDragon(fragCoord+d.xy)+
      TwinDragon(fragCoord-d.xy)+
      TwinDragon(fragCoord+d.yx)+
      TwinDragon(fragCoord-d.yx)
  )*.25;
    
	fragColor = vec4(col,1.);
}
