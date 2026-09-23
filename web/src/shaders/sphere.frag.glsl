uniform float uVolume;
uniform float uOnset;
uniform float uSpectralFlux;

varying vec3 vNormal;
varying vec3 vWorldPosition;

void main() {
  vec3 viewDir = normalize(cameraPosition - vWorldPosition);
  float facing = max(0.0, dot(viewDir, normalize(vNormal)));

  // Inverse fresnel: the core burns white at the centre and fades to cyan
  // before reaching the silhouette, so no hard surface is ever visible.
  float body = pow(facing, 2.2);
  float hot = pow(facing, 5.0);

  vec3 cyan = vec3(0.08, 0.72, 1.0);
  vec3 white = vec3(0.95, 0.99, 1.0);
  vec3 color = mix(cyan, white, hot);

  float intensity = body * (0.24 + uVolume * 0.18 + uOnset * 0.2 + uSpectralFlux * 0.08);
  gl_FragColor = vec4(color * intensity, 1.0);
}
