# Índice de Conocimiento de Negocio

Este archivo es el mapa que usa el MCP para saber **dónde buscar**. Lo lee
`get_entity` y `search_database`/`search_knowledge` lo usan como una de sus
fuentes. Editarlo (o añadir archivos en `knowledge/business/`) es la forma de
enseñarle algo nuevo al MCP **sin recompilar el binario**: basta con guardar
el archivo y volver a llamar a la tool `refresh_schema` o reiniciar el
proceso, que relee este índice en cada consulta.

Convención de cada entidad:

```
### <Nombre>
- tablas: tabla1, tabla2, tabla3
- documento: archivo.md            (dentro de knowledge/business/)
- descripcion: una frase
- relacion:<verbo>: <OtraEntidad>  (relación semántica explícita, no inferida)
```

Solo se incluyen aquí las entidades que ya tienen evidencia estructural
(tablas confirmadas por introspección real de `database_multi_final`, MySQL
5.7). El significado de negocio de columnas como `estado`, códigos y reglas
específicas **todavía no está documentado**: es exactamente lo que este MCP
debe ir aprendiendo, sesión a sesión, vía `record_observation` →
`propose_knowledge` → `approve_knowledge`. No hay que inventarlo aquí.

## Entidades

### Venta
- tablas: pedidosventa, detallepedido, factura_venta, detallefactura_venta, cliente
- documento: ventas.md
- descripcion: Proceso de venta; pendiente de documentar reglas de negocio (estados, tipos, cortesías, etc.) mediante el flujo de aprendizaje.
- relacion:pertenece: Cliente
- relacion:genera: Factura

### Cliente
- tablas: cliente, categoriaclientes, consultorcliente
- documento: clientes.md
- descripcion: Entidad cliente del sistema.

### Inventario
- tablas: almacen, productoalmacen, kardex, kardex_snapshot_diario
- documento: inventario.md
- descripcion: Control de stock y movimientos de almacén.
- relacion:afectado_por: Venta

### Caja
- tablas: caja, cajas, cajadiaria, cajadiariamovimiento, cajamovimiento, flujocaja
- documento: caja.md
- descripcion: Movimientos y arqueo de caja/cobros.

### CajaChica
- tablas: cajachica, aperturacajachica, caja_chica_notas, caja_chica_pagos, caja_chica_items
- documento: caja.md
- descripcion: Subsistema de caja chica (gastos menores), separado de la caja principal.
