attribute float aSeed;
attribute float aAccent;

uniform float uTime;
uniform float uEnergy;
uniform float uBass;
uniform float uPulse;
uniform float uReveal;
uniform float uAlert;
uniform float uPixelRatio;

varying float vAlpha;
varying vec3 vColor;

void main() {
  vec3 p = position * (1.0 + uBass * 0.02 + uPulse * 0.015);
  vec4 mvPosition = modelViewMatrix * vec4(p, 1.0);
  gl_Position = projectionMatrix * mvPosition;

  vec3 viewNormal = normalize((modelViewMatrix * vec4(normalize(position), 0.0)).xyz);
  float rim = 1.0 - abs(viewNormal.z);
  float front = smoothstep(-0.25, 0.25, viewNormal.z);
  float twinkle = 0.7 + 0.3 * sin(uTime * 1.7 + aSeed * 60.0);

  vAlpha = (0.1 + rim * 0.38) * mix(0.3, 1.0, front) * twinkle * (1.0 + uEnergy * 0.8 + uPulse * 0.4);
  vAlpha = mix(vAlpha, min(1.0, vAlpha * 2.2), aAccent) * uReveal;

  vec3 base = mix(vec3(0.28, 0.6, 1.0), vec3(0.85, 0.3, 0.75), uAlert);
  vec3 accent = mix(vec3(1.0, 0.42, 0.85), vec3(1.0, 0.6, 0.4), uAlert);
  vColor = mix(base, accent, aAccent);

  gl_PointSize = (1.9 + aAccent) * uPixelRatio * (7.0 / -mvPosition.z);
}
