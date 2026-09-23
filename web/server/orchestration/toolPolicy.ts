export type ToolExecutionPolicy = 'AUTO' | 'CONFIRMATION_REQUIRED' | 'DENIED';

const AUTO_EXECUTABLE_TOOLS = new Set<string>([
  'recordar_contexto',
  'obtener_conocimiento_negocio',
  'buscar_conocimiento',
  'obtener_conocimiento_validado',
  'obtener_entidad',
  'obtener_relaciones',
  'obtener_esquema_bd',
  'obtener_esquema_tabla',
  'buscar_en_base_datos',
  'consultar_base_datos',
  'explicar_consulta',
  'obtener_conocimiento_pendiente',
  'obtener_estadisticas_uso',
]);

const CONFIRMATION_REQUIRED_TOOLS = new Set<string>([
  'refrescar_esquema',
  'registrar_observacion',
  'proponer_conocimiento',
  'aprender_del_usuario',
]);

const DENIED_TOOLS = new Set<string>([
  'aprobar_conocimiento',
  'rechazar_conocimiento',
  'actualizar_alias',
]);

export function getToolPolicy(toolName: string): ToolExecutionPolicy {
  if (AUTO_EXECUTABLE_TOOLS.has(toolName)) {
    return 'AUTO';
  }
  if (CONFIRMATION_REQUIRED_TOOLS.has(toolName)) {
    return 'CONFIRMATION_REQUIRED';
  }
  if (DENIED_TOOLS.has(toolName)) {
    return 'DENIED';
  }
  // Unknown or unverified tool defaults to DENIED
  return 'DENIED';
}
