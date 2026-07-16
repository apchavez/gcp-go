[![CI](https://github.com/apchavez/gcp-go/actions/workflows/ci.yml/badge.svg)](https://github.com/apchavez/gcp-go/actions/workflows/ci.yml)
[![Quality Gate Status](https://sonarcloud.io/api/project_badges/measure?project=apchavez_gcp-go&metric=alert_status)](https://sonarcloud.io/summary/new_code?id=apchavez_gcp-go)
[![Coverage](https://sonarcloud.io/api/project_badges/measure?project=apchavez_gcp-go&metric=coverage)](https://sonarcloud.io/summary/new_code?id=apchavez_gcp-go)

# Plataforma de Agendamiento de Citas Médicas — GCP

Plataforma backend para agendamiento de citas médicas construida con **Go** y **Google Cloud Platform**, usando Clean Architecture.

Este es el **tercer hermano** del mismo dominio de agendamiento de citas, junto a:

| Proyecto | Cloud / Lenguaje |
|---|---|
| [aws-typescript](https://github.com/apchavez/aws-typescript) | AWS Lambda / TypeScript |
| [azure-python](https://github.com/apchavez/azure-python) | Azure Functions / Python |
| **gcp-go** (este repo) | GCP Cloud Run / Go |

Mismo dominio de negocio, mismos 5 endpoints, mismas reglas de autorización por titularidad del asegurado, y los mismos números de JWT hecho a mano (HS256) y resiliencia retry/circuit-breaker que sus dos hermanos — solo cambian el cloud y el lenguaje, a propósito, para demostrar que las mismas capacidades de ingeniería son portables entre ecosistemas.

Fue desplegado en vivo a un proyecto real de GCP (Cloud Run + Firestore + Pub/Sub + Cloud SQL) y probado de punta a punta — crear, obtener historial, y listado paginado fueron verificados contra la API desplegada — y luego destruido vía `destroy.yml` para evitar costo ocioso. `deploy.yml` vuelve a desplegar el mismo stack bajo demanda con una corrida manual.

> **Costo cero en reposo** — el CI solo compila y corre pruebas. No se aprovisiona ningún recurso de GCP hasta que se dispara manualmente el workflow de deploy.

---

## Stack Tecnológico

| Capa | Tecnología |
|---|---|
| Lenguaje | Go 1.25 |
| Runtime | GCP Cloud Run (2 servicios: `api`, `worker`) |
| Router de API | `net/http` + `chi` |
| Almacén de estado | Firestore (modo Native) — colecciones `appointments` + `appointment-events` |
| Almacén relacional | Cloud SQL para PostgreSQL (solo citas finales/completadas) |
| Mensajería | Pub/Sub (tópicos `appointment-created`/`completed`/`cancelled`, suscripciones push filtradas por atributo de país) |
| Notificaciones | SendGrid (best-effort; no-op si no está configurado) |
| Auth | JWT HS256 hecho a mano, autorización por titularidad del asegurado |
| Resiliencia | Retry + circuit breaker hecho a mano (3 intentos, backoff 100/200/400ms, ventana de 10 llamadas, umbral 50%, 30s abierto, 3 sondas half-open) |
| IaC | Terraform |
| Testing | `testing` de Go + `testify`, tests table-driven |
| Docs | OpenAPI 3.1 |

## Arquitectura

```mermaid
flowchart TD
    Client([Cliente]) -->|POST /appointments| API[Cloud Run: api]
    API -->|guarda PENDING| FS[(Firestore: appointments)]
    API -->|publica, atributo country| PS[Pub/Sub: appointment-created]
    API -->|agrega evento| FSE[(Firestore: appointment-events)]
    PS -->|push filtrado, country=PE| WPE[Cloud Run: worker]
    PS -->|push filtrado, country=CL| WPE
    WPE -->|marca COMPLETED| FS
    WPE -->|persiste| SQL[(Cloud SQL: appointments)]
    WPE -->|notifica| SG[SendGrid]
```

El backend sigue **Clean Architecture / Hexagonal (Ports & Adapters)**:

```
gcp-go/
├── cmd/
│   ├── api/            Entrypoint de la API HTTP (servicio Cloud Run)
│   └── worker/         Worker de suscripción push de Pub/Sub (servicio Cloud Run separado)
├── internal/
│   ├── domain/          Appointment, AppointmentEvent, ports (interfaces), errores de dominio
│   ├── application/     AppointmentService — los 6 casos de uso
│   ├── infrastructure/
│   │   ├── auth/         verificación/firma de JWT hecha a mano + guard de auth
│   │   ├── resilience/   retry + circuit breaker hecho a mano
│   │   ├── repos/        repo de estado en Firestore, event store en Firestore, repo en Cloud SQL
│   │   ├── messaging/    publisher de Pub/Sub + handler push del worker
│   │   └── notifications/ notificador de SendGrid + fallback no-op
│   ├── api/              capa de handlers HTTP (routing, validación, auth, mapeo de errores)
│   └── shared/           helpers de respuesta HTTP, estado de salud
├── db/migration/        esquema de Cloud SQL
├── terraform/           Cloud Run, Firestore, Pub/Sub, Cloud SQL, Secret Manager, IAM
├── api/openapi.yaml      especificación OpenAPI 3.1
└── postman/              colección de Postman + environments
```

## API

| Método | Ruta | Descripción |
|---|---|---|
| POST | `/appointments` | Crea una nueva cita (estado `pending`) |
| GET | `/appointments/{insuredId}` | Lista citas por asegurado, paginado (`pageSize`, `cursor`) |
| DELETE | `/appointments/{appointmentUuid}` | Cancela una cita pendiente |
| PATCH | `/appointments/{appointmentUuid}/reschedule` | Reagenda una cita pendiente |
| GET | `/appointments/{appointmentUuid}/history` | Historial completo de eventos de una cita |
| GET | `/health` | Health check (anónimo) |

Especificación completa: [`api/openapi.yaml`](api/openapi.yaml).

### Autorización

Token Bearer JWT, `HS256`, claims `{sub, role, iat, exp}`. El rol `insured` solo puede actuar sobre su propio `insuredId` (comparado contra `sub`); el rol `agent` no tiene restricción. Esto se aplica de forma idéntica en los 4 endpoints que reciben un identificador de cita/asegurado, incluyendo `GET .../history` — que primero busca la cita para verificar titularidad, en vez de derivar el dueño a partir del primer evento (un detalle sutil que de hecho fue un bug real en los handlers de historial de Java/Python de este dominio, corregido en esta sesión).

## Desarrollo local

```bash
export GCP_PROJECT_ID=your-project
export JWT_SECRET=local-dev-secret
go run ./cmd/api
```

Requiere Application Default Credentials (`gcloud auth application-default login`) para acceso a Firestore/Pub/Sub cuando se corre contra recursos reales de GCP, o apuntar a los emuladores de Firestore/Pub/Sub para desarrollo completamente offline.

## Testing

```bash
go test ./... -race -coverprofile=coverage.out
go vet ./...
golangci-lint run ./...
```

Las capas de dominio + aplicación tienen un gate de cobertura del 80% (refleja los gates de JaCoCo/pytest-cov de los hermanos AWS/Azure); los adaptadores de infraestructura están testeados pero no tienen gate, ya que envuelven clientes reales del SDK de GCP.

## Infraestructura

`terraform/` provisiona: 2 servicios Cloud Run (api, worker), Firestore (modo Native) con índices compuestos, 3 tópicos Pub/Sub + 2 suscripciones push filtradas por país + un tópico dead-letter, Cloud SQL para PostgreSQL, Secret Manager (secreto JWT, key de SendGrid, password de Cloud SQL), y una service account dedicada con bindings IAM de mínimo privilegio.

El CI de este repo no provisiona ningún proyecto GCP en vivo — `deploy.yml`/`destroy.yml` son solo `workflow_dispatch` y requieren credenciales de GCP configuradas como secretos del repositorio.

`cost-guard.yml` corre diariamente (06:00 UTC), sin necesidad de activarlo manualmente — revisa el timestamp de creación del servicio Cloud Run `api`, y si tiene más de 48h (configurable vía `max_age_hours` en una corrida manual), dispara `destroy.yml` él mismo vía la API de GitHub. No hace nada si no hay nada desplegado. Existe para que un deploy de demostración nunca siga facturando en silencio días después.

Ciclo completo deploy→smoke test→destroy verificado en vivo el 2026-07-15: `deploy.yml` aprovisiona Cloud Run (api+worker), Cloud SQL, Pub/Sub, Secret Manager y Firestore, corre un smoke test real (`/health` y un endpoint autenticado con JWT firmado en el propio job) contra la URL desplegada, y `destroy.yml` deja confirmado cero recursos facturables — incluyendo las imágenes gcr.io, cuya limpieza requiere que la service account del deployer (`github-deployer@clinic-scheduling-gcp-dev.iam.gserviceaccount.com`) tenga `roles/datastore.owner` y `roles/artifactregistry.repoAdmin` a nivel de proyecto (otorgados el 2026-07-15; sin esos dos roles, `terraform apply` falla al crear Firestore y `destroy.yml` deja hasta 2 imágenes huérfanas de costo despreciable). La base Firestore `(default)` no se puede eliminar vía API una vez creada (solo deshabilitar) — `deploy.yml` la reimporta al estado de Terraform en cada corrida en vez de intentar recrearla.

## Proyectos Relacionados

Este repo hace pareja con **aws-typescript** y **azure-python**: los tres implementan el mismo dominio de agendamiento de citas y Clean Architecture, los mismos 5 endpoints, distinto cloud/lenguaje — mantenidos en paridad funcional a propósito. Los cuatro proyectos fullstack de Kubernetes forman un segundo grupo así, compartiendo un dominio de Gestión de Productos en su lugar.

| Proyecto | Descripción |
|---|---|
| [aws-typescript](https://github.com/apchavez/aws-typescript) | La versión original en AWS — TypeScript, Lambda, DynamoDB, SNS/SQS. Misma lógica de dominio, distinto cloud |
| [azure-python](https://github.com/apchavez/azure-python) | Migración a Azure de esta plataforma — mismo dominio y Clean Architecture, reescrito en **Python** sobre Azure Functions, Cosmos DB, y Service Bus |
| [quarkus-react](https://github.com/apchavez/quarkus-react) | Plataforma de Gestión de Productos — backend Quarkus, frontend React, MongoDB, Redis, eventos Kafka, Kubernetes |
| [spring-webflux-angular](https://github.com/apchavez/spring-webflux-angular) | Mismo dominio de Gestión de Productos que arriba, backend reactivo Spring Boot WebFlux, frontend Angular, PostgreSQL, Kafka, Kubernetes |
| [spring-mvc-angular](https://github.com/apchavez/spring-mvc-angular) | Mismo dominio de Gestión de Productos y frontend Angular que spring-webflux-angular, backend clásico bloqueante Spring MVC, Spring Data JDBC, Kafka, Kubernetes |
| [net-vue](https://github.com/apchavez/net-vue) | Mismo dominio de Gestión de Productos, backend ASP.NET Core, frontend Vue 3, PostgreSQL, Kafka, Kubernetes |

## Qué Demuestra Este Proyecto

- Límites de Clean Architecture / hexagonal en Go idiomático (interfaces como ports, sin magia de framework)
- Una tercera implementación independiente del mismo dominio de agendamiento de citas orientado a eventos, probando que el diseño se traduce entre AWS, Azure, y GCP
- Verificación de JWT hecha a mano y una implementación de resiliencia hecha a mano (retry + circuit breaker), portada con parámetros idénticos entre 3 lenguajes
- Patrones cloud-native de GCP: Firestore para estado rápido + event sourcing, filtrado de mensajes por atributo en Pub/Sub como mecanismo de ruteo por país, Cloud SQL para el lado relacional durable, Secret Manager + IAM de mínimo privilegio
- IaC con Terraform, CI con GitHub Actions con un gate de cobertura acotado, contrato documentado con OpenAPI, y una colección de Postman mantenida en sincronía con los proyectos hermanos
