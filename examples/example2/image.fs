
void mainImage( out vec4 fragColor, in vec2 fragCoord )
{
  fragColor = texture(iChannel0, fragCoord.xy/iResolution.xy) * texture(iChannel1, fragCoord.xy/iResolution.xy) * texture(iChannel2, fragCoord.xy/iResolution.xy);
}