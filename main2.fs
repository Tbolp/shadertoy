
void mainImage( out vec4 fragColor, in vec2 fragCoord )
{
fragColor = texture(iChannel0, fragCoord/600);
// fragColor = vec4(1.0, 0.0, 0.0, 1.0);
}