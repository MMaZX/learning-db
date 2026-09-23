precision highp float;

varying float vAlpha;

void main() {
  gl_FragColor = vec4(0.32, 0.66, 1.0, vAlpha);
}
