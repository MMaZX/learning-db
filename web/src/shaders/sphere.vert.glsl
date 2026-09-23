uniform float uFlow;
uniform float uEnergy;
uniform float uBass;
uniform float uMid;
uniform float uPulse;

varying vec3 vNormal;
varying vec3 vWorldPosition;

void main() {
  // Bounded displacement (max ~+0.2): the heart glows brighter, it does not inflate.
  vec3 noiseCoord = position * 1.2 + vec3(uFlow * 1.5);
  float displacement =
      snoise(noiseCoord) * (0.03 + uBass * 0.05)
    + snoise(noiseCoord * 2.4 - vec3(uFlow * 4.0)) * (uMid * 0.03)
    + uEnergy * 0.06
    + uPulse * 0.04;

  vec3 deformed = position + normal * displacement;
  vec4 worldPos = modelMatrix * vec4(deformed, 1.0);
  vWorldPosition = worldPos.xyz;
  vNormal = normalize(mat3(modelMatrix) * normal);

  gl_Position = projectionMatrix * viewMatrix * worldPos;
}
