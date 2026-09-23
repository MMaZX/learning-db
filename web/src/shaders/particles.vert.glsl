attribute float aSeed;
attribute float aSize;
attribute float aLayer;
attribute float aStrand;
attribute vec3 aVelocity;

uniform float uFlow;
uniform float uEnergy;
uniform float uBass;
uniform float uMid;
uniform float uTreble;
uniform float uPulse;
uniform float uBootDots;
uniform float uBootFull;
uniform float uAlert;
uniform float uCoreRadius;
uniform float uPixelRatio;

varying float vAlpha;
varying vec3 vColor;

// Soft ceiling: identity below `knee`, then eases asymptotically towards `limit`,
// so pushing the audio harder never lets a particle leave the core envelope.
float softLimit(float r, float knee, float limit) {
  if (r <= knee) return r;
  float room = limit - knee;
  return knee + room * (1.0 - exp(-(r - knee) / room));
}

void main() {
  vec3 base = position;
  float r0 = length(base);
  vec3 n = base / max(r0, 1e-4);
  float t = uFlow;

  // Boot: points first, then they stretch into filaments.
  float grow = smoothstep(0.0, 0.45, uBootFull);

  // Each strand is a quadratic curve that follows the noise flow on the
  // tangent plane, so neighbouring points read as a continuous filament.
  float strandLen = 0.05 + (0.16 + aSeed * 0.16 + uEnergy * 0.12) * grow;
  if (aLayer > 1.5) strandLen *= 0.5;

  vec3 flowOffset = vec3(0.0, t, t * 0.5);
  vec3 t0 = tangentFlow(base * 0.85 + flowOffset, n);
  vec3 mid = base + t0 * strandLen * 0.5;
  vec3 t1 = tangentFlow(mid * 0.85 + flowOffset, n);
  vec3 tip = mid + t1 * strandLen * 0.5;

  float s = aStrand;
  vec3 curve = mix(mix(base, mid, s), mix(mid, tip, s), s);

  // Small, bounded radial motion: the reference barely grows (+10–20 %).
  float lift = aLayer < 0.5 ? 0.3 : (aLayer < 1.5 ? 1.0 : 1.8);
  float ripple = snoise(n * 2.3 + vec3(t * 1.6)) * (0.025 + uMid * 0.06);
  float swell = (uBass * 0.05 + uEnergy * 0.06) * lift;
  float burst = uPulse * 0.08 * lift * (0.4 + aSeed);
  vec3 p = curve + n * (ripple + swell + burst);

  float turbulence = uTreble * 0.08 + uPulse * 0.05;
  p += aVelocity * snoise(base * 3.1 + vec3(t * 6.0)) * turbulence;

  float limit = uCoreRadius * (aLayer > 1.5 ? 1.3 : 1.15);
  float pr = length(p);
  p *= softLimit(pr, uCoreRadius, limit) / max(pr, 1e-4);

  vec4 mvPosition = modelViewMatrix * vec4(p, 1.0);
  gl_Position = projectionMatrix * mvPosition;

  float taper = 0.45 + 0.55 * sin(s * 3.14159);
  gl_PointSize = aSize * taper * (1.0 + uEnergy * 0.35) * uPixelRatio * (300.0 / -mvPosition.z);

  vec3 cyan = vec3(0.10, 0.88, 1.0);
  vec3 blue = vec3(0.12, 0.52, 1.0);
  vec3 white = vec3(0.88, 0.97, 1.0);
  vec3 ash = vec3(0.72, 0.76, 0.82);
  vec3 magenta = vec3(1.0, 0.22, 0.62);
  vec3 rose = vec3(1.0, 0.5, 0.78);

  float inner = 1.0 - smoothstep(0.2, 1.35, r0);
  vec3 col = mix(mix(cyan, blue, aSeed * 0.4), mix(magenta, rose, aSeed * 0.5), uAlert);
  col = mix(col, white, inner * 0.35 + uPulse * 0.15);
  if (aLayer > 1.5) col = mix(mix(cyan, magenta, uAlert), white, 0.45);
  // Grey-white while booting, tinted as the spectrum comes alive.
  col = mix(ash, col, smoothstep(0.1, 0.6, uBootFull));
  vColor = col;

  // The inner volume stacks up along the line of sight at the centre, so it
  // stays dim to keep the filaments readable instead of a white blob.
  float layerAlpha = aLayer < 0.5 ? 0.26 : (aLayer < 1.5 ? 0.62 : 0.7);
  float presence = 0.45 + 0.55 * grow;
  vAlpha = layerAlpha * taper * presence * (0.85 + uEnergy * 0.35 + uPulse * 0.2);
  if (aLayer > 1.5) vAlpha *= clamp(0.35 + uEnergy, 0.0, 1.0);

  // Dots fade in one by one on page load.
  vAlpha *= smoothstep(aSeed, aSeed + 0.1, uBootDots * 1.1);
}
