Quiero implementar un sistema de visualización 3D de voz en tiempo real para una aplicación web.

IMPORTANTE:
No quiero un mock.
No quiero una animación pregrabada.
No quiero un ecualizador de barras.
No quiero una esfera que simplemente cambie de escala.

Necesito un visualizador 3D REAL que utilice el micrófono del usuario y que reaccione en tiempo real a las características de su voz.

La estética buscada es similar a un "digital audio organism":

- esfera/núcleo energético
- miles de partículas
- núcleo extremadamente luminoso
- partículas alrededor del núcleo
- pequeñas partículas que se desprenden
- deformación procedural
- turbulencia
- glow cyan/azul
- comportamiento orgánico
- sensación de inteligencia artificial / energía digital
- fondo azul muy oscuro/casi negro

La referencia visual principal son las imágenes proporcionadas en la conversación.

============================================================
1. STACK TECNOLÓGICO OBLIGATORIO
============================================================

Utilizar:

- React
- TypeScript
- Vite
- Three.js
- Web Audio API
- WebGL 2
- GLSL shaders

NO utilizar React Three Fiber inicialmente.

La razón es que quiero mantener control directo sobre:

- render loop
- shaders
- BufferGeometry
- uniforms
- partículas
- postprocessing
- lifecycle de Three.js
- rendimiento

React debe utilizarse únicamente para:

- UI
- controles
- estados de aplicación
- lifecycle/orquestación
- interacción del usuario

NO utilizar React state para actualizar datos del audio 60 veces por segundo.

El flujo debe ser:

React
    ↓
AudioAnalyzer
    ↓
mutable audio data
    ↓
Three.js
    ↓
GPU / shaders

NO:

Audio
    ↓
setState()
    ↓
React rerender
    ↓
Three.js

============================================================
2. ARQUITECTURA
============================================================

Crear una arquitectura modular.

Una estructura aproximada:

src/

  components/
    AudioVisualizer.tsx

  audio/
    AudioAnalyzer.ts

  three/
    AudioScene.ts
    AudioSphere.ts
    ParticleField.ts
    OrbitField.ts
    PostProcessing.ts

  shaders/
    sphere.vert.glsl
    sphere.frag.glsl
    particles.vert.glsl
    particles.frag.glsl

  types/
    audio.ts

  hooks/
    useAudioVisualizer.ts

  App.tsx
  main.tsx

La arquitectura existente del proyecto debe inspeccionarse antes de modificar archivos.

NO asumir que esta estructura exacta existe.

Si el proyecto ya tiene una estructura equivalente, respetarla.

============================================================
3. RESPONSABILIDADES
============================================================

React:

- montar/desmontar visualizador
- mostrar estados
- botón de activar micrófono
- mostrar errores
- controles de desarrollo
- integrar posteriormente estados de conversación

AudioAnalyzer:

- solicitar micrófono
- crear AudioContext
- crear MediaStreamAudioSourceNode
- crear AnalyserNode
- procesar FFT
- calcular características de audio
- smoothing
- noise floor
- noise gate
- onset/transient detection

Three.js:

- escena
- cámara
- renderer
- geometrías
- partículas
- shaders
- iluminación/emission
- bloom
- resize
- render loop

Los componentes deben estar desacoplados.

============================================================
4. API PÚBLICA
============================================================

El sistema debe poder utilizarse conceptualmente así:

const visualizer = new AudioVisualizer(container);

await visualizer.startMicrophone();

visualizer.start();

visualizer.stop();

visualizer.destroy();

Si se utiliza una clase diferente por la arquitectura React, mantener igualmente una API limpia.

El componente React debería poder utilizarse aproximadamente así:

<AudioVisualizer />

o:

<AudioVisualizer
    audioAnalyzer={audioAnalyzer}
/>

============================================================
5. MICRÓFONO REAL
============================================================

Utilizar:

navigator.mediaDevices.getUserMedia({
    audio: true
});

NO usar archivos de audio.

NO simular audio.

NO generar valores aleatorios para reemplazar el micrófono.

El micrófono debe controlar realmente la visualización.

Solicitar permiso únicamente después de una interacción explícita del usuario.

Estado inicial:

IDLE

Al pulsar:

"Activar micrófono"

pasar a:

REQUESTING_PERMISSION

Después:

LISTENING

Si falla:

ERROR

Manejar correctamente:

- Permission denied
- microphone unavailable
- getUserMedia no soportado
- AudioContext suspendido
- stream terminado
- dispositivo desconectado si resulta posible

Si AudioContext está suspendido:

await audioContext.resume();

============================================================
6. AUDIO ANALYSIS
============================================================

Utilizar:

AudioContext

MediaStreamAudioSourceNode

AnalyserNode

Configurar inicialmente:

fftSize = 2048

Utilizar:

getByteFrequencyData()

y:

getByteTimeDomainData()

o equivalentes Float32 cuando resulte conveniente.

Crear un modelo de datos:

interface AudioFeatures {

    volume: number;
    smoothedVolume: number;

    bass: number;
    lowMid: number;
    mid: number;
    highMid: number;
    treble: number;

    spectralFlux: number;
    onset: number;
}

Todos los valores destinados al renderer deben estar normalizados aproximadamente entre:

0.0 → 1.0

============================================================
7. OPTIMIZACIÓN PARA VOZ HUMANA
============================================================

Esto NO es un visualizador musical.

El principal input será voz humana.

Debe funcionar correctamente con:

- voz grave
- voz aguda
- voz normal
- habla rápida
- habla lenta
- voz fuerte
- voz suave
- susurros razonablemente detectables

No asumir que el usuario está reproduciendo música.

Analizar bandas aproximadamente dentro del rango vocal.

No utilizar límites rígidos que hagan que determinadas voces no reaccionen.

Como referencia conceptual:

Bass:
~80-180 Hz

Low-mid:
~180-500 Hz

Mid:
~500-2000 Hz

High-mid:
~2000-4000 Hz

Treble:
~4000-8000 Hz

Los rangos deben ser configurables.

============================================================
8. NOISE FLOOR
============================================================

El ruido del micrófono no debe provocar que el organismo permanezca permanentemente activo.

Al iniciar el micrófono:

realizar una calibración de aproximadamente:

500-1000 ms

Durante ese período:

medir el nivel ambiental.

Determinar:

noiseFloor

Después:

effectiveVolume =
    max(0, rawVolume - noiseFloor)

Normalizar posteriormente.

Agregar también:

noiseGate

para ignorar pequeñas fluctuaciones.

============================================================
9. SMOOTHING
============================================================

No utilizar directamente el valor bruto de FFT.

Implementar smoothing separado para:

- attack
- release

Ejemplo conceptual:

const audioConfig = {

    sensitivity: 1.5,

    smoothing: 0.8,

    noiseGate: 0.03,

    attack: 0.25,

    release: 0.12

};

Los valores deben poder ajustarse.

La animación debe sentirse física y orgánica.

NO debe producir:

jitter

flickering

micro movimientos nerviosos causados por ruido.

============================================================
10. BANDAS DE FRECUENCIA
============================================================

Separar al menos:

LOW
LOW-MID
MID
HIGH-MID
HIGH

El visualizador debe utilizar cada banda para algo diferente.

VOLUME:

- energía general
- expansión
- glow
- intensidad

BASS:

- expansión radial
- deformación fuerte
- pulsación
- movimiento de partículas grandes

MID:

- deformación orgánica
- turbulencia
- desplazamiento de superficie

HIGH:

- partículas pequeñas
- microactividad
- pequeños destellos

SPECTRAL FLUX / ONSET:

- pulsos
- emisión de partículas
- pequeñas explosiones radiales
- aumento momentáneo del glow

============================================================
11. ESFERA PRINCIPAL
============================================================

No utilizar un modelo 3D externo.

Construir la esfera proceduralmente.

Puede utilizarse:

THREE.IcosahedronGeometry

con suficiente subdivisión.

La deformación debe ejecutarse principalmente en shader.

La superficie debe utilizar:

- procedural noise
- time
- vertex normal
- audio bands

Conceptualmente:

displacement =
    noise(position, time)
        * baseNoiseAmplitude

    + bass
        * bassAmplitude

    + mid
        * midAmplitude

    + high
        * highAmplitude;

El ruido debe existir incluso cuando no haya audio.

En silencio:

la esfera está viva pero tranquila.

Cuando aparece voz:

aumenta la energía.

============================================================
12. PROHIBICIÓN IMPORTANTE
============================================================

NO hacer:

sphere.scale.setScalar(volume);

como mecanismo principal.

La escala global puede existir ligeramente, pero no debe ser el comportamiento dominante.

La visualización debe sentirse como un organismo deformándose internamente.

============================================================
13. PARTICLE FIELD
============================================================

Crear un sistema de partículas GPU-friendly.

Utilizar:

THREE.BufferGeometry

THREE.Points

ShaderMaterial

NO crear miles de Mesh individuales.

Objetivo inicial:

3000-10000 partículas.

La cantidad debe ser configurable.

Cada partícula debe tener como mínimo:

- position
- random seed
- size
- phase
- velocity/procedural movement parameter

Las partículas deben formar una estructura aproximadamente esférica.

NO quiero una nube aleatoria sin forma.

Deben parecer parte del organismo.

============================================================
14. CAPAS DE PARTÍCULAS
============================================================

Utilizar conceptualmente tres regiones:

1. CORE

Partículas muy cercanas al núcleo.

2. SURFACE

Partículas alrededor de la superficie.

3. OUTER FIELD

Partículas más alejadas.

La densidad debe variar.

Esto debe crear profundidad visual.

============================================================
15. PARTÍCULAS DESPRENDIDAS
============================================================

Cuando:

bass

volume

o

onset

aumenten:

algunas partículas pueden abandonar temporalmente el núcleo.

Comportamiento:

1. nacen cerca de la superficie
2. reciben velocidad radial
3. se alejan
4. reciben turbulencia
5. pierden energía
6. regresan o desaparecen
7. son reemplazadas

No crear entidades JavaScript individuales por partícula.

Mantener la mayor parte de este comportamiento en GPU/procedural.

============================================================
16. MOVIMIENTO PROCEDURAL
============================================================

Utilizar ruido procedural.

La animación debe incluir:

- curl/turbulence
- movimiento orbital
- drift
- radial displacement
- pequeñas variaciones de velocidad

Incluso en silencio debe existir movimiento.

Pero el movimiento debe ser mucho más intenso cuando existe voz.

============================================================
17. ORBITAL ELEMENTS
============================================================

Añadir algunos elementos orbitales sutiles.

No deben parecer anillos planetarios.

Deben parecer:

- trayectorias holográficas
- líneas digitales
- fragmentos orbitales
- interfaces alrededor del organismo

Deben ser secundarios.

No deben dominar la composición.

============================================================
18. MATERIAL
============================================================

Paleta:

cyan
electric blue
white core

Fondo:

dark navy / almost black

La esfera debe utilizar:

- additive blending cuando corresponda
- Fresnel
- emission
- bloom
- transparencia controlada

Evitar:

- lava
- fuego
- humo
- galaxia
- agua
- planeta

El resultado debe parecer:

DIGITAL
ENERGETIC
INTELLIGENT
HOLOGRAPHIC
ORGANIC

============================================================
19. BLOOM
============================================================

Implementar postprocessing de Three.js.

Bloom moderado.

El bloom debe reaccionar ligeramente al volumen.

Conceptualmente:

bloomStrength =
    baseBloom +
    smoothedVolume * audioBloomAmount;

Aplicar clamp.

NO saturar toda la pantalla.

El núcleo debe ser la región más luminosa.

============================================================
20. CÁMARA
============================================================

Utilizar una cámara que haga que la esfera se sienta grande y central.

Inicialmente:

PerspectiveCamera

con framing cinematográfico.

La cámara puede tener un movimiento extremadamente sutil:

- slight drift
- very slow orbital movement

pero NO debe marear.

============================================================
21. IDLE STATE
============================================================

Cuando no haya voz:

NO congelar.

Debe existir:

- movimiento procedural
- partículas flotantes
- ruido
- glow mínimo
- movimiento orbital
- respiración muy lenta

Estado:

IDLE

debe sentirse como:

"la entidad está esperando".

============================================================
22. LISTENING STATE
============================================================

Cuando el usuario habla:

LISTENING

El organismo aumenta su actividad.

La intensidad depende realmente del micrófono.

No cambiar de estado solamente porque el usuario pulsó un botón.

El estado debe poder calcularse a partir del audio:

effectiveVolume > threshold

============================================================
23. FUTURO ESTADO SPEAKING
============================================================

La arquitectura debe permitir posteriormente una segunda fuente de audio:

IA / TTS

Por eso NO acoplar el AudioAnalyzer únicamente al micrófono.

Diseñar una abstracción conceptual:

AudioSource
    ↓
AudioFeatures
    ↓
Visualizer

De esta manera posteriormente podremos tener:

MicrophoneAudioSource

y:

TTSAudioSource

sin reconstruir el visualizador.

Por ahora implementar únicamente:

MicrophoneAudioSource

pero dejar la arquitectura preparada.

============================================================
24. AUDIO DATA NO DEBE PASAR POR REACT
============================================================

IMPORTANTE.

No hacer:

setAudioFeatures(...)

en cada frame.

No almacenar FFT en React state.

El flujo debe ser mutable/directo:

AudioAnalyzer
    ↓
AudioFeatures object/ref
    ↓
Three.js uniforms
    ↓
GPU

React únicamente controla:

- start
- stop
- configuration
- UI state

============================================================
25. RENDER LOOP
============================================================

Cada frame:

1. actualizar audio
2. calcular AudioFeatures
3. aplicar smoothing
4. actualizar uniforms
5. actualizar partículas
6. actualizar postprocessing
7. renderizar

Evitar allocations.

NO hacer dentro del loop:

new Vector3()
new Color()
new Array()
new BufferGeometry()

etc.

Reutilizar objetos.

============================================================
26. PERFORMANCE
============================================================

Objetivo:

60 FPS cuando sea posible.

Debe funcionar en:

- laptops con GPU integrada
- desktops modernos
- Chrome
- Edge
- Firefox

Limitar:

pixelRatio = Math.min(window.devicePixelRatio, 2);

Si el rendimiento cae:

reducir progresivamente:

1. bloom quality
2. pixel ratio
3. particle count

No reducir todo inmediatamente.

Crear una estrategia simple de quality scaling si resulta razonable.

============================================================
27. WEBGL
============================================================

Utilizar WebGL 2 como renderer principal.

No depender de WebGPU para la primera versión.

El objetivo es máxima compatibilidad.

WebGPU puede evaluarse posteriormente si necesitamos simulaciones de partículas mucho más grandes.

============================================================
28. RESPONSIVE
============================================================

El visualizador debe funcionar dentro de cualquier contenedor.

No asumir:

window.innerWidth

como única referencia.

Utilizar el tamaño real del container.

Preferiblemente:

ResizeObserver

para detectar cambios.

Actualizar:

- renderer
- camera
- postprocessing
- viewport

============================================================
29. CLEANUP
============================================================

Al desmontar React component:

debe limpiarse correctamente:

- animation frame
- AudioContext
- MediaStream
- tracks
- Three.js renderer
- geometries
- materials
- textures
- postprocessing
- event listeners
- ResizeObserver

No debe haber memory leaks.

============================================================
30. PRIVACIDAD
============================================================

El audio del micrófono:

NO debe enviarse al servidor.

NO debe grabarse.

NO debe almacenarse.

NO debe subirse.

Todo el análisis visual debe ejecutarse localmente.

============================================================
31. DEBUG MODE
============================================================

Implementar:

debug = true

que permita mostrar:

FPS
Volume
Bass
LowMid
Mid
HighMid
Treble
Spectral Flux
Onset
Noise Floor

Crear controles de desarrollo para:

Sensitivity
Noise Gate
Attack
Release
Smoothing
Particle Count
Bloom
Idle Energy
Bass Response
Mid Response
High Response

Estos controles pueden ser simples.

No es necesario construir una UI final.

============================================================
32. UX
============================================================

Inicialmente mostrar:

"Activate microphone"

Después de permiso:

"Listening"

Mientras exista voz:

"Listening..."

Cuando esté en silencio:

"Waiting..."

Los textos son secundarios.

El visualizador debe ser el protagonista.

============================================================
33. COMPATIBILIDAD CON FUTURA IA
============================================================

Diseñar desde ahora pensando en:

USER SPEAKING

        ↓

Microphone
        ↓
AudioAnalyzer
        ↓
AudioFeatures
        ↓
Visualizer


AI SPEAKING

        ↓

TTS Audio
        ↓
AudioAnalyzer
        ↓
AudioFeatures
        ↓
Visualizer

La capa visual NO debe saber de dónde viene el audio.

============================================================
34. IMPLEMENTACIÓN POR FASES
============================================================

No intentes construir todo simultáneamente.

FASE 1:

Implementar:

Microphone
→ AudioContext
→ AnalyserNode
→ FFT
→ Volume

Verificar que el volumen REAL cambia al hablar.

FASE 2:

Agregar:

Bass
LowMid
Mid
HighMid
Treble

Verificar valores mediante debug.

FASE 3:

Crear esfera procedural.

Verificar deformación.

FASE 4:

Conectar audio a shaders.

FASE 5:

Crear particle field.

FASE 6:

Conectar partículas al audio.

FASE 7:

Agregar desprendimiento de partículas.

FASE 8:

Agregar bloom.

FASE 9:

Agregar orbital elements.

FASE 10:

Optimización y cleanup.

FASE 11:

Integración final con React.

============================================================
35. CRITERIOS DE ACEPTACIÓN
============================================================

La implementación NO se considera terminada si solamente existe una animación bonita.

Debe cumplirse:

[ ] El usuario puede activar el micrófono.

[ ] El navegador solicita permiso.

[ ] AudioContext funciona.

[ ] FFT funciona.

[ ] Volume cambia cuando el usuario habla.

[ ] Bass/Mid/High cambian según el audio.

[ ] El ruido ambiental no produce actividad excesiva.

[ ] La esfera se deforma realmente según el audio.

[ ] Las partículas reaccionan realmente según el audio.

[ ] Los agudos producen comportamiento diferente a los graves.

[ ] Onset produce pequeños eventos visuales.

[ ] En silencio existe idle animation.

[ ] No se envía audio al backend.

[ ] No se utiliza React state por frame.

[ ] El render utiliza GPU.

[ ] El sistema funciona en Chrome/Edge/Firefox modernos.

[ ] Resize funciona.

[ ] El componente se puede desmontar sin memory leaks.

[ ] TypeScript compila sin errores.

[ ] No existen errores de runtime evidentes.

============================================================
36. REGLA FUNDAMENTAL
============================================================

PRIMERO FUNCIONALIDAD.

Antes de perfeccionar la estética:

MIC
↓
FFT
↓
AudioFeatures
↓
Three.js
↓
Shader
↓
Visual reaction

debe funcionar realmente.

Después mejorar:

- estética
- partículas
- shaders
- glow
- composición
- movimiento
- calidad visual

No quiero que se dedique tiempo a crear una esfera espectacular que luego no reacciona realmente al micrófono.

============================================================
37. AL FINAL
============================================================

Entregar un resumen indicando:

1. archivos creados
2. archivos modificados
3. dependencias agregadas
4. arquitectura final
5. cómo funciona el pipeline de audio
6. cómo se conectan las bandas FFT con los shaders
7. cómo iniciar el proyecto
8. cómo probar el micrófono
9. problemas encontrados
10. decisiones técnicas importantes

Si encuentras una limitación del proyecto existente, detente y explícala antes de inventar una solución.

No inventes APIs.

No inventes dependencias.

Inspecciona primero el proyecto existente y adapta la implementación a su arquitectura.
