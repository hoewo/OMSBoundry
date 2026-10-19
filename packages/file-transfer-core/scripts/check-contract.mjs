import { readFile } from 'node:fs/promises'

const contractUrl = new URL('../../../contracts/evidence/v1/openapi.json', import.meta.url)
const contract = JSON.parse(await readFile(contractUrl, 'utf8'))

if (contract.openapi !== '3.1.0') throw new Error('Evidence contract must use OpenAPI 3.1.0')
if (!String(contract.info?.version ?? '').startsWith('1.')) {
  throw new Error('Evidence contract must remain on explicit major version 1')
}

const requiredPaths = [
  '/v1/upload-sessions',
  '/v1/upload-sessions/{session_id}/complete',
  '/v1/upload-sessions/{session_id}/abort',
  '/v1/files/{file_id}/versions/{version}/access-grants',
]
for (const path of requiredPaths) {
  if (!contract.paths?.[path]) throw new Error(`Evidence contract is missing ${path}`)
}

const requestProperties = contract.components?.schemas?.CreateUploadSessionRequest?.properties ?? {}
for (const forbidden of ['project_id', 'category', 'role', 'permission', 'button']) {
  if (Object.hasOwn(requestProperties, forbidden)) {
    throw new Error(`Evidence upload contract must not expose business field ${forbidden}`)
  }
}

const authorization = contract.components?.schemas?.UploadAuthorization
if (authorization?.discriminator?.propertyName !== 'provider' || authorization?.oneOf?.length !== 2) {
  throw new Error('Upload authorization must be a provider-discriminated union')
}

const aliyunAuthorization = contract.components?.schemas?.AliyunOssUploadAuthorization
const requiredAliyunAuthorizationFields = [
  'object_key',
  'security_token',
  'expires_at',
  'multipart',
  'scope',
]
for (const field of requiredAliyunAuthorizationFields) {
  if (!aliyunAuthorization?.required?.includes(field)) {
    throw new Error(`Aliyun upload authorization must require ${field}`)
  }
}

const localAuthorization = contract.components?.schemas?.LocalHttpUploadAuthorization
for (const field of ['upload_url', 'method', 'object_key', 'expires_at', 'scope']) {
  if (!localAuthorization?.required?.includes(field)) {
    throw new Error(`Local upload authorization must require ${field}`)
  }
}

if (contract.components?.schemas?.CreateUploadSessionRequest?.additionalProperties !== false) {
  throw new Error('CreateUploadSessionRequest must fail closed on unknown fields')
}

console.log('Evidence v1 contract guard passed')
