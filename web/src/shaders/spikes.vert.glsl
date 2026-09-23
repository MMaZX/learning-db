attribute float aEnd;
attribute float aSeed;

uniform float uTime;
uniform float uRadius;
uniform float uVolume;
uniform float uBass;
uniform float uTreble;
uniform float uOnset;

varying float vAlpha;

float hash(float n) { return fract(sin(n) * 43758.5453123); }

void main() {
  vec3 dir = normalize(position);

  // Re-roll each spike ~12 times per second for a crackling spectrum feel.
  float flicker = hash(aSeed * 91.7 + floor(uTime * 12.0 + aSeed * 7.0) * 0.37);
  float energy = uVolume * 0.9 + uOnset * 1.3 + uTreble * 0.5;
  float len = energy * (0.04 + 0.6 * flicker * flicker);

  float shell = uRadius * (1.0 + uBass * 0.06 + uOnset * 0.04);
  vec3 p = dir * (shell + aEnd * len);

  vec4 mvPosition = modelViewMatrix * vec4(p, 1.0);
  gl_Position = projectionMatrix * mvPosition;

  vAlpha = (1.0 - aEnd * 0.85) * clamp(energy * 1.4, 0.0, 1.0) * 0.75;
}
