# Ventas

> Estado: **documentación inicial (hechos estructurales verificados por
> introspección de schema)**. El significado de negocio de columnas como
> `estado`, `pendiente`, `tipoventa`, `Boletafactura` etc. NO está validado
> todavía. No asumir su significado: usar `record_observation` y
> `propose_knowledge` para construirlo con evidencia real, y `approve_knowledge`
> para validarlo humanamente antes de tratarlo como un hecho.

## Tablas principales (hecho, confirmado en `database_multi_final`, MySQL 5.7)

- `pedidosventa` — cabecera del pedido/venta. PK `codPedido`. Columnas
  relevantes: `codCliente`, `codAlmacen`, `codTipoDocumento`, `codCotizacion`,
  `estado` (bit), `pendiente` (bit), `tipoventa`, `total`, `bruto`,
  `montodscto`, `igv`, `fechapedido`, `fechaentrega`, `codUsuario`,
  `codEmpresa`, `codSerie`, `seriedoc`.
- `detallepedido` — líneas del pedido (productos).
- `factura_venta` / `detallefactura_venta` — facturación asociada a la venta.
- `cotizacion` / `cotizaciones` / `cotizacion_pdoc` — cotizaciones, posible
  origen de un pedido vía `codCotizacion`.
- `salidaventa`, `detalleventasalida`, `detallesalidaventa` — salida física
  de mercadería asociada a la venta (posible vínculo con inventario/kardex,
  pendiente de confirmar con `get_relationships`).

## Relaciones confirmadas por FK (hecho, ver `get_relationships`)

Consultar `get_relationships("pedidosventa")` para obtener el listado exacto
y actualizado de FKs salientes/entrantes: este documento no las duplica para
evitar que queden desactualizadas si el schema cambia (usar `refresh_schema`).

## Pendiente de aprender (usar el flujo de observación/propuesta/validación)

- Significado de cada valor de `pedidosventa.estado` y `pedidosventa.pendiente`.
- Qué distingue `tipoventa` (valores observados hasta ahora: ninguno todavía).
- Relación exacta entre `pedidosventa`, `cotizacion` y `salidaventa` en el
  flujo operativo real (¿toda venta pasa por cotización? ¿toda venta genera
  salida de almacén?).
- Qué representa `Boletafactura` y `codTipoDocumento` en términos de
  documentos SUNAT (boleta/factura/nota).

Cuando se acumule evidencia suficiente sobre alguno de estos puntos, generar
una propuesta con `propose_knowledge` citando las queries/observaciones que
la sustentan.
