attribute float aSeed;
attribute float aWarm;

uniform float uTime;
uniform float uEnergy;
uniform float uBootDots;
uniform float uBootFull;
uniform float uAlert;
uniform float uPixelRatio;

varying float vAlpha;
varying vec3 vColor;

void main() {
  // Slow radial breathing, bounded to a few percent of the orbit.
  float breathe = 1.0 + 0.04 * sin(uTime * (0.3 + aSeed * 0.4) + aSeed * 40.0);
  vec4 mvPosition = modelViewMatrix * vec4(position * breathe, 1.0);
  gl_Position = projectionMatrix * mvPosition;

  float twinkle = 0.55 + 0.45 * sin(uTime * (1.2 + aSeed * 2.5) + aSeed * 90.0);
  float presence = uBootDots * (0.45 + 0.55 * uBootFull);
  vAlpha = presence * twinkle * (0.45 + uEnergy * 0.55);

  vec3 cool = mix(vec3(0.3, 0.95, 1.0), vec3(1.0, 0.35, 0.75), uAlert);
  vec3 warm = mix(vec3(1.0, 0.25, 0.45), vec3(1.0, 0.7, 0.35), uAlert);
  vColor = mix(cool, warm, aWarm);

  gl_PointSize = (1.4 + aSeed * 2.2) * (1.0 + uEnergy * 0.3) * uPixelRatio * (7.0 / -mvPosition.z);
}
