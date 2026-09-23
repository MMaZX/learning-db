precision highp float;

varying float vAlpha;
varying vec3 vColor;

void main() {
  float dist = length(gl_PointCoord - vec2(0.5));
  if (dist > 0.5) discard;

  float glow = smoothstep(0.5, 0.0, dist);
  gl_FragColor = vec4(vColor, vAlpha * glow);
}
