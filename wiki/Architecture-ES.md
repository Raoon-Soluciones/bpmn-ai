# Arquitectura

---

## Resumen

El Motor BPMN-AI está estructurado en tres capas principales.

### Capa 1: API HTTP (`api/http/`)

Construida sobre chi router, proporciona endpoints RESTful y una pila de middleware.

### Capa 2: Motor Central (`internal/engine/`)

El motor utiliza un **bucle de ejecución iterativo no recursivo**. Los workers procesan elementos desde un canal, el router determina los siguientes elementos, y el failsafe manager impone límites de seguridad. Un registro de elementos (patrón factory) resuelve los tipos de elementos.

### Capa 3: Elementos BPMN (`internal/element/`)

19+ tipos de elementos que implementan la interfaz `BPMNElement`. El método `Execute()` retorna una de varias acciones:

| Acción | Significado |
|---|---|
| `ActionRoute` | Proceder al siguiente elemento normalmente |
| `ActionWait` | Pausar ejecución, instancia → `WAITING` |
| `ActionForm` | UserTask — pausar para finalización externa |
| `ActionQueue` | ServiceTask — asíncrono vía cola de trabajos |
| `ActionThrowError` | ErrorEndEvent — propagar para captura de error |
| `ActionTerminate` | TerminateEvent — detener toda ejecución |

### Transversal: Sistema IA (`pkg/ai/`)

El módulo IA proporciona integración con LLM para el elemento `AITask`, con ruteo de modelos, RAG, guardrails, caché y llamada a herramientas.

---

## Flujo de Ejecución

```
Petición HTTP → Engine.Iterate() → ExecuteElement()
    → Router.DetermineNext() → HandleResult() → [loop]
```

1. El motor recibe una solicitud de inicio
2. Crea una ProcessInstance en estado `CREATED`
3. Transiciona a `IN_PROGRESS`
4. Entra en bucle iterativo:
   a. Ejecuta el elemento actual
   b. Maneja el resultado (route, wait, queue, error, terminate)
   c. Determina el/los siguiente(s) elemento(s) vía router
   d. Programa eventos de borde si es necesario
   e. Repite hasta que no haya más elementos o esté WAITING/ERROR
5. Instancia → `COMPLETED` cuando todos los hilos terminan

---

## Sub-Proceso y CallActivity

**Sub-Proceso**: XML interno parseado via `xml:",innerxml"`, los elementos se aplanan en el proceso padre con prefijo `{subID}.{origID}`. Un flujo sintético de entrada `{id}_sp_entry` enruta hacia el sub-proceso.

**CallActivity**: Parsea el atributo `calledElement`. Carga el proceso llamado desde el store, aplana elementos con prefijo `ca-{id}.`, crea flujo sintético de entrada, ejecuta internamente, y enruta de vuelta al completarse.

---

## Hilos y Estado de Instancia

- **CREATED**: Estado inicial tras crear la instancia
- **IN_PROGRESS**: Siendo ejecutada activamente por el motor
- **WAITING**: Pausada en UserTask, MessageCatch, SignalCatch o evento de borde
- **SUSPENDED**: Suspendida explícitamente por API
- **ERROR**: Error irrecuperable
- **TERMINATED**: Detenida por TerminateEvent o API
- **COMPLETED**: Todos los hilos terminaron exitosamente

Fórmula de índice de hilo: `parentThreadIdx * 10 + branchIndex + 1`

---

## Eventos de Borde

Los eventos de borde se adjuntan a actividades. Cuando una actividad entra en estado `WAITING` o `FORM`, `scheduleBoundaryTimers()` crea registros de flujo para:
- **Borde Timer**: Crea un trabajo programado por `CalculateSchedule()`
- **Borde Message**: Crea un registro de flujo activo encontrado por `SendMessage()`
- **Borde Error**: Encontrado por `findErrorCatch()` al lanzar error

Los bordes interruptores cancelan flujos adjuntos. Los no interruptores (`cancelActivity="false"`) se disparan sin cancelación.

---

## Cola de Trabajos (`internal/queue/`)

La cola de trabajos maneja ejecución asíncrona:
- Pool de workers con concurrencia configurable
- Reintento con retroexponencial
- Cola de mensajes muertos para trabajos fallidos

Usado por ServiceTask, Timer catch events y bordes timer programados.
