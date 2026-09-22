# Inventario / Kardex

> Estado: documentación inicial, hechos estructurales únicamente.

## Tablas (hecho)

- `almacen` — almacenes/sedes de stock.
- `productoalmacen` — stock de producto por almacén (probable tabla de
  existencias actuales; confirmar con `get_table_schema`).
- `kardex` — movimientos de inventario (entradas/salidas). Es el registro
  histórico de movimientos; nombre coincide con el término contable/logístico
  estándar "kardex" pero su estructura exacta (columnas, tipos de movimiento)
  debe confirmarse con `get_table_schema("kardex")` antes de asumir nada.
- `kardex_snapshot_diario` — probable snapshot diario de stock (a confirmar).
- `almacen_requerimiento` / `almacen_requerimiento_detalle` /
  `almacen_requerimiento_historial` — requerimientos de almacén (posible
  flujo de transferencia/reposición entre almacenes).

## Pendiente de aprender

- Qué operaciones de venta/compra generan filas en `kardex` (relación con
  `pedidosventa`, `salidaventa`, compras — pendiente de localizar tablas de
  compra).
- Tipos de movimiento en kardex (entrada/salida/ajuste) y su codificación.
- Diferencia funcional entre `kardex` y `kardex_snapshot_diario`.

Este es el área de mayor prioridad para el flujo de aprendizaje según lo
indicado por el usuario: enseñar aquí primero, iterando con
`record_observation` → `propose_knowledge` → `approve_knowledge`.
