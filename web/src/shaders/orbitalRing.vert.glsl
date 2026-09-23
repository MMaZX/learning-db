attribute float aAngle;
attribute float aAccent;

uniform float uDraw;
uniform float uBootFull;
uniform float uEnergy;
uniform float uAlert;
uniform float uPixelRatio;

varying float vAlpha;
varying vec3 vColor;

void main() {
  vec4 mvPosition = modelViewMatrix * vec4(position, 1.0);
  gl_Position = projectionMatrix * mvPosition;

  // Drawn in from the top on page load, like the arc at t=0.3 s in the reference.
  float drawn = smoothstep(aAngle - 0.02, aAngle, uDraw);
  // Prominent while booting, then settles into a faint orbit.
  float level = mix(0.55, 0.22, smoothstep(0.6, 1.0, uBootFull)) * (1.0 + uEnergy * 0.6);
  vAlpha = drawn * level * (aAccent > 0.5 ? 1.8 : 1.0);

  vec3 base = mix(vec3(0.62, 0.72, 1.0), vec3(1.0, 0.45, 0.8), uAlert);
  vec3 accent = mix(vec3(0.3, 1.0, 0.95), vec3(1.0, 0.7, 0.4), uAlert);
  vColor = mix(base, accent, aAccent);

  gl_PointSize = (aAccent > 0.5 ? 3.2 : 1.8) * uPixelRatio * (7.0 / -mvPosition.z);
}
