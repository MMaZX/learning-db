attribute float aEnd;
attribute float aHead;
attribute float aSeed;

uniform float uTime;
uniform float uStart;
uniform float uMaxLength;
uniform float uEnergy;
uniform float uPulse;
uniform float uTreble;
uniform float uReveal;
uniform float uAlert;

varying float vAlpha;
varying vec3 vColor;

float hash(float n) { return fract(sin(n) * 43758.5453123); }

void main() {
  vec3 dir = normalize(position);

  // Each spike blends between two random lengths ~2.5 times per second, so the
  // crown crackles without frame-to-frame jitter.
  float clock = uTime * 2.5 + aSeed * 7.0;
  float k = floor(clock);
  float a = hash(aSeed * 91.7 + k * 0.37);
  float b = hash(aSeed * 91.7 + (k + 1.0) * 0.37);
  float rnd = mix(a, b, smoothstep(0.0, 1.0, fract(clock)));

  float drive = clamp(uEnergy * 1.2 + uPulse * 0.4 + uTreble * 0.15, 0.0, 1.0) * uReveal;
  // Only part of the crown is lit at any time; the rest stays retracted.
  float lit = smoothstep(0.5, 0.7, rnd);
  float reach = uMaxLength * (1.0 + uAlert * 0.25);
  float len = reach * drive * lit * (0.35 + 0.65 * rnd);

  vec3 p = dir * (uStart + aEnd * len);
  vec4 mvPosition = modelViewMatrix * vec4(p, 1.0);
  gl_Position = projectionMatrix * mvPosition;

  vec3 bodyInner = mix(vec3(0.15, 0.5, 1.0), vec3(0.85, 0.2, 0.6), uAlert);
  vec3 bodyTip = mix(vec3(1.0, 0.3, 0.55), vec3(1.0, 0.55, 0.3), uAlert);
  vec3 headColor = mix(vec3(0.35, 1.0, 0.85), vec3(1.0, 0.8, 0.92), uAlert);
  vColor = aHead > 0.5 ? headColor : mix(bodyInner, bodyTip, aEnd);

  float visible = drive * smoothstep(0.02, 0.1, len);
  vAlpha = aHead > 0.5 ? visible * 0.8 : visible * (0.45 - aEnd * 0.15);
}
