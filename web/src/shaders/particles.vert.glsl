attribute float aSeed;
attribute float aSize;
attribute float aPhase;
attribute float aLayer;
attribute float aStrand;
attribute vec3 aVelocity;

uniform float uTime;
uniform float uVolume;
uniform float uBass;
uniform float uLowMid;
uniform float uMid;
uniform float uHighMid;
uniform float uTreble;
uniform float uOnset;
uniform float uPixelRatio;

varying float vAlpha;
varying vec3 vColor;

void main() {
  float energy = clamp(uVolume * 0.7 + uBass * 0.5 + uOnset * 0.9, 0.0, 1.6);
  float t = uTime * (0.12 + energy * 0.3);

  float spin = uTime * 0.06;
  mat2 rot = mat2(cos(spin), -sin(spin), sin(spin), cos(spin));
  vec3 base = position;
  base.xz = rot * base.xz;

  float r = length(base);
  vec3 n = base / max(r, 1e-4);

  // Each strand is a quadratic curve that follows the noise flow on the
  // tangent plane, so neighbouring points read as a continuous filament.
  float strandLen = 0.2 + aSeed * 0.16 + energy * 0.18;
  if (aLayer > 1.5) strandLen *= 0.5;

  vec3 flowCoord = base * 0.85 + vec3(0.0, t, t * 0.5);
  vec3 t0 = tangentFlow(flowCoord, n);
  vec3 mid = base + t0 * strandLen * 0.5;
  vec3 t1 = tangentFlow(mid * 0.85 + vec3(0.0, t, t * 0.5), n);
  vec3 tip = mid + t1 * strandLen * 0.5;

  float s = aStrand;
  vec3 curve = mix(mix(base, mid, s), mix(mid, tip, s), s);

  // Surface ripple (voice timbre) + burst (onsets / loud peaks)
  float ripple = snoise(n * 2.3 + vec3(t * 1.6)) * (0.03 + uMid * 0.2 + uLowMid * 0.12);
  float bassSwell = uBass * 0.22;

  float burstGain = aLayer < 0.5 ? 0.35 : (aLayer < 1.5 ? 0.8 : 2.6);
  float burst = (uOnset * 0.8 + uVolume * 0.45) * burstGain * (0.25 + aSeed * 1.1);

  vec3 p = curve + n * (ripple + bassSwell + burst);

  float turbulence = uTreble * 0.35 + uHighMid * 0.2 + uOnset * 0.3;
  p += aVelocity * snoise(base * 3.1 + vec3(uTime * 2.4)) * turbulence;

  vec4 mvPosition = modelViewMatrix * vec4(p, 1.0);
  gl_Position = projectionMatrix * mvPosition;

  float taper = 0.45 + 0.55 * sin(s * 3.14159);
  gl_PointSize = aSize * taper * (1.0 + uVolume * 0.5) * uPixelRatio * (300.0 / -mvPosition.z);

  vec3 cyan = vec3(0.10, 0.88, 1.0);
  vec3 blue = vec3(0.12, 0.52, 1.0);
  vec3 white = vec3(0.88, 0.97, 1.0);

  float inner = 1.0 - smoothstep(0.2, 1.35, r);
  vec3 col = mix(cyan, blue, aSeed * 0.4);
  col = mix(col, white, inner * 0.55 + uOnset * 0.25);
  if (aLayer > 1.5) col = mix(cyan, white, 0.45);
  vColor = col;

  float layerAlpha = aLayer < 0.5 ? 0.5 : (aLayer < 1.5 ? 0.62 : 0.7);
  vAlpha = layerAlpha * taper * (0.85 + uVolume * 0.5 + uOnset * 0.4);
  if (aLayer > 1.5) vAlpha *= clamp(0.35 + energy, 0.0, 1.0);
}
