uniform float uTime;
uniform float uVolume;
uniform float uBass;
uniform float uMid;
uniform float uTreble;
uniform float uOnset;

varying vec3 vNormal;
varying vec3 vWorldPosition;

void main() {
  vec3 noiseCoord = position * 1.2 + vec3(uTime * 0.2);
  float displacement =
      snoise(noiseCoord) * (0.04 + uBass * 0.25)
    + snoise(noiseCoord * 2.4 - vec3(uTime * 0.6)) * (uMid * 0.12)
    + uOnset * 0.18
    + uVolume * 0.12;

  vec3 deformed = position + normal * displacement;
  vec4 worldPos = modelMatrix * vec4(deformed, 1.0);
  vWorldPosition = worldPos.xyz;
  vNormal = normalize(mat3(modelMatrix) * normal);

  gl_Position = projectionMatrix * viewMatrix * worldPos;
}
