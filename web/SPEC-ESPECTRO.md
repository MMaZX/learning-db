# Especificación del espectro (referencia SAO)

Fuente: `contenido_espectro.mp4` (23.85 s, 1920×1080, 60 fps contenedor, ~25 fps dibujados).
Solo se analiza el espectro; personajes, subtítulos y fondo azul de sala se ignoran.
Métricas medidas por frame (escenas de fondo oscuro, 0.8–4.6 s y 13–18 s) con escala 480×270.

## 1. Mediciones del video

| Medida | Reposo (1–2.8 s) | Activo cian (3.4–4.6 s) | Activo naranja (13–18 s) |
|---|---|---|---|
| Centro (x, y) | 241, 136–142 | 240, 145–147 | 237, 140–142 |
| Radio núcleo r50 | 49–51 | 55–60 (+15 %) | 66–75 |
| Radio brillante r90 | 64–70 | 86–96 | 91–106 |
| Alcance máx. r99 (picos) | 88–96 | 113–123 | 122–137 |
| Energía de movimiento | ~1.0 | ~2.3–3.5 | ~2.3–5.7 |
| Rotación global estimada | ~2 °/s | ~9 °/s (mayormente flujo turbulento) | ~12 °/s (mayormente flujo) |

Conclusiones:
- **El centro no se desplaza** (±1 px). La esfera nunca "sale disparada".
- **El núcleo casi no crece**: +10–20 % entre reposo y activo. La energía se expresa con brillo,
  densidad, picos radiales y color, no inflando la esfera.
- **Los picos llegan como máximo a ~1.3–1.4× el radio del núcleo** en estado normal.
  Solo en el estado de "alerta" (magenta, 19.8 s+) los elementos invaden la pantalla.
- **El movimiento es suave**: la energía de movimiento sube y baja en rampas de ~0.3–0.5 s,
  sin saltos de un frame a otro.
- **La superficie no gira como sólido rígido** (correlación de fase 0.04–0.07): es un flujo
  de filamentos turbulento sobre una rotación global lenta y constante.

## 2. Anatomía del espectro (de adentro hacia afuera)

1. **Corazón caliente**: resplandor blanco en el centro; crece en brillo con la energía.
2. **Volumen de filamentos**: miles de trazos cortos tipo "pelo/fibra" que fluyen tangencialmente.
   Denso y lleno: no hay huecos negros en el disco.
3. **Piel del núcleo**: borde irregular y peludo (no una silueta limpia), ligeramente más densa.
4. **Retícula punteada** (~1.25–1.35× núcleo): esfera de puntos en malla lat/long, visible sobre todo en el borde.
5. **Decals HUD** tangentes a la retícula: anillos enlazados, glifo "LW", arco grueso brillante,
   clúster de anillos, corchetes laterales. Giran con la esfera.
6. **Picos radiales** (1.1× → 1.6× núcleo): trazos discontinuos con un segmento brillante
   y cola; distribuidos por toda la esfera, longitudes variadas.
7. **Chispas flotantes** exteriores: puntos cian y magenta dispersos a 1.3–2× radio, deriva lenta.
8. **Anillo orbital punteado** (~1.6× núcleo): visible en el arranque, gira lento.
9. **Aberración cromática** global (separación RGB en los bordes) y bloom.

## 3. Secuencia de encendido (0 → 4 s)

| t | Qué ocurre |
|---|---|
| 0.0–0.5 s | Aparecen puntos blancos dispersos formando la esfera completa (tamaño final desde el inicio). Arco punteado empieza a dibujarse arriba. |
| 0.5–1.0 s | Los puntos se estiran en filamentos gris-blancos; el anillo orbital punteado se completa con puntos cian. |
| 1.0–2.8 s | Densidad y brillo suben gradualmente; tinte pasa de gris a cian. Rotación casi nula. |
| ~3.0 s | Aparece la retícula punteada y los decals HUD (fade-in). |
| 3.4–4.0 s | Primera activación: corazón blanco, picos radiales, flujo acelerado. |

## 4. Estados de color observados

| Estado | Color dominante | Momento |
|---|---|---|
| Arranque | blanco/gris | 0–1 s |
| Normal | cian | 1–8 s |
| Transición | cian-verde → verde-lima | 8–13 s |
| Voz desconocida | naranja/rosado | 16–19.5 s |
| Alerta / ruptura | magenta con anillos, invade pantalla | 19.8 s+ |

El color depende del **estado**, no del volumen instantáneo.

## 5. Diagnóstico del código actual (`web/src`)

### Por qué el espectro "se escapa"
1. **Desplazamientos que se apilan sin techo** (`shaders/particles.vert.glsl:48-57`):
   `burst` para la capa 2 = `(onset·0.8 + volume·0.45) · 2.6 · (0.25 + seed·1.1)` → hasta **~4.4 u**
   sobre un núcleo de 1.5 u (casi 4× radio). La capa 1 llega a ~1.35 u → rebasa la retícula (2.2 u).
   Se suman además `bassSwell` (0.22·bass, con bass hasta 1.4), `ripple` y `turbulence`.
2. **Features crudas por frame**: `volume`, `bass`, `mid`, `treble`, `onset` llegan a los shaders
   sin suavizar (solo `smoothedVolume` pasa por el `AsymmetricSmoother`).
3. **`onset` con ataque instantáneo** (0 → 0.5–1.0 en un frame) y caída `×0.82` por frame.
4. **Sensibilidad lineal sin compresión**: `(rms − floor) · 1.2 · 2.2` satura a 1.0 enseguida;
   `bassResponse` 1.4 deja `bass` en 1.4 sin clamp.

### Por qué la velocidad no es constante
5. **Fase de tiempo escalada** (`particles.vert.glsl:23`): `t = uTime · (0.12 + energy·0.3)`.
   Al cambiar `energy`, `t` salta `uTime·Δ` (a los 100 s, Δ=0.5 → salto de 15 unidades):
   el campo de ruido se "teletransporta" y la velocidad parece errática. Debe integrarse en CPU
   (`phase += dt · speed(energy)`).
6. **Suavizados dependientes del framerate**: coeficientes por frame (60 Hz vs 144 Hz se ven distintos).
7. `HoloShell` sí integra la rotación con `delta` (correcto); las partículas rotan con `uTime·0.06` (fijo).

### Por qué "no está lleno alrededor"
8. Faltan: chispas flotantes exteriores, anillo orbital punteado, picos con segmento brillante
   (hoy son líneas continuas de color plano), aberración cromática, corazón caliente más marcado.
9. No existe secuencia de encendido ni estados de color.

## 6. Objetivos de la nueva versión

- Envolvente dura: ninguna partícula del núcleo pasa de ~1.15× núcleo; picos ≤ 1.6× núcleo.
- Todas las features pasan por un suavizado con ataque/relajación en **segundos** (dt-based) y
  una curva de compresión suave (soft-knee) antes de llegar al render.
- Velocidades integradas en CPU (fase acumulada), rotación global lenta y constante.
- Energía → brillo, densidad visible, longitud de picos, bloom. No → tamaño del núcleo (máx. +15 %).
- Capas nuevas: chispas exteriores, anillo orbital, picos con cabeza brillante, aberración cromática.
- Secuencia de encendido en dos etapas: al cargar la página, puntos blancos dispersos (etapa 0–1 s del video);
  al encender el micrófono, se completa (filamentos → cian → retícula/HUD → activación).
- Paleta: cian siempre; magenta de alerta solo en errores (micrófono o servidor), transición suave ~1 s.
