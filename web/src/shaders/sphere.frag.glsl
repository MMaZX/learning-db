uniform float uEnergy;
uniform float uPulse;
uniform float uBootFull;
uniform float uAlert;

varying vec3 vNormal;
varying vec3 vWorldPosition;

void main() {
  vec3 viewDir = normalize(cameraPosition - vWorldPosition);
  float facing = max(0.0, dot(viewDir, normalize(vNormal)));

  // Inverse fresnel: the core burns white at the centre and fades to cyan
  // before reaching the silhouette, so no hard surface is ever visible.
  float body = pow(facing, 3.0);
  float hot = pow(facing, 5.0);

  vec3 tint = mix(vec3(0.08, 0.72, 1.0), vec3(1.0, 0.3, 0.7), uAlert);
  vec3 white = vec3(0.95, 0.99, 1.0);
  vec3 color = mix(tint, white, hot);

  // The heart only lights up once the spectrum is booted; energy drives brightness.
  float intensity = body * (0.02 + uBootFull * 0.04 + uEnergy * 0.1 + uPulse * 0.05);
  gl_FragColor = vec4(color * intensity, 1.0);
}
