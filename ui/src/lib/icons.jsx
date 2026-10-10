// Resource type -> sprite id, following the design system's alias map. The
// sprite itself is inlined in index.html so <use href="#ri-…"> resolves.
const ALIAS = {
  'http-client': 'http', http: 'http', 'http-server': 'http-server',
  'grpc-client': 'grpc', grpc: 'grpc',
  'websocket-client': 'websocket', websocket: 'websocket',
  'websocket-server': 'websocket-server',
  postgresql: 'postgres', postgres: 'postgres',
  cassandra: 'scylladb', scylladb: 'scylladb',
  minio: 's3', s3: 's3',
  redis: 'redis', kafka: 'kafka', rabbitmq: 'rabbitmq',
  shell: 'shell', aws: 'aws',
}

export const iconId = (type) => ALIAS[String(type || '').toLowerCase()] || 'app'

export function ResourceIcon({ type, ...rest }) {
  return (
    <svg className="ri" aria-hidden="true" {...rest}>
      <use href={`#ri-${iconId(type)}`} />
    </svg>
  )
}
