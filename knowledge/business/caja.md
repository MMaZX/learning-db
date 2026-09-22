# Caja y Caja Chica

> Estado: documentación inicial, hechos estructurales únicamente.

## Caja principal (hecho)

- `caja`, `cajas` — definición de cajas.
- `cajadiaria`, `cajadiariamovimiento` — apertura/cierre diario y sus
  movimientos.
- `cajamovimiento` — movimientos de caja en general.
- `flujocaja` — probable vista/consolidado de flujo de caja.
- `aperturacierre`, `arqueo`, `arqueofondofijo` — apertura/cierre y arqueo de
  caja.

## Caja chica (hecho, subsistema separado con su propio prefijo `caja_chica_*`)

- `cajachica`, `aperturacajachica`
- `caja_chica_notas`, `caja_chica_notas_detalle`, `caja_chica_notas_historial`,
  `caja_chica_notas_facturacion`
- `caja_chica_pagos`, `caja_chica_items`
- `caja_chica_proveedores`, `caja_chica_proveedor_config`
- `caja_chica_tipo_documento`, `caja_chica_transiciones_log`,
  `caja_chica_turnos_config`, `caja_chica_usuarios_autorizados`

El prefijo `caja_chica_*` sugiere un módulo más nuevo/estructurado que la
caja principal (más tablas de configuración y control de transiciones/estado
— `caja_chica_transiciones_log` en particular sugiere una máquina de estados
explícita). Esto es una hipótesis razonable a partir del nombrado, NO un
hecho validado: registrar como hypothesis con `record_observation` si se
confirma explorando los datos.

## Pendiente de aprender

- Relación entre `cajadiaria`/`cajamovimiento` y `pedidosventa`/`factura_venta`
  (cómo se registra el cobro de una venta en caja).
- Estados válidos en `caja_chica_transiciones_log` y su máquina de estados.
