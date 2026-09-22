# Clientes

> Estado: documentación inicial, hechos estructurales únicamente.

## Tablas (hecho)

- `cliente` — entidad cliente.
- `categoriaclientes` — categorización de clientes (posible segmentación
  comercial; significado exacto pendiente de validar).
- `consultorcliente` — probable relación cliente–consultor/vendedor asignado
  (pendiente de confirmar con evidencia real vía `get_table_schema` y
  observación de datos).
- `clientes_fletes` — datos de flete asociados a clientes (envíos).

## Pendiente de aprender

- Reglas de segmentación en `categoriaclientes`.
- Relación operativa `consultorcliente` (¿asignación fija? ¿por venta?).

Usar `get_table_schema("cliente")` para ver columnas exactas antes de asumir
nada, y `record_observation` para dejar constancia de lo que se vaya
descubriendo en sesiones futuras.
