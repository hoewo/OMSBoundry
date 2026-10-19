import {
  FileTransferClient,
  FileTransferError,
  createAliOssMultipartUploader,
  createDirectUploaderRouter,
  createFetchFileTransferApi,
  createLocalHttpUploader,
  type FileDescriptor,
  type UploadFileResult,
} from '@omsbizfoundry/file-transfer-core'
import './style.css'

const templateTenantId = '33333333-3333-4333-8333-333333333333'
const templateSubjectId = '44444444-4444-4444-8444-444444444444'

const api = createFetchFileTransferApi({ baseUrl: '/evidence' })
const client = new FileTransferClient({
  api,
  uploader: createDirectUploaderRouter({
    aliyunOss: createAliOssMultipartUploader(),
    localHttp: createLocalHttpUploader(),
  }),
  getCacheScope: () => `${templateTenantId}:${templateSubjectId}`,
})

const elements = {
  fileInput: element<HTMLInputElement>('file-input'),
  smallSample: element<HTMLButtonElement>('small-sample'),
  largeSample: element<HTMLButtonElement>('large-sample'),
  fileSummary: element<HTMLElement>('file-summary'),
  ownerRef: element<HTMLInputElement>('owner-ref'),
  purpose: element<HTMLInputElement>('purpose'),
  upload: element<HTMLButtonElement>('upload'),
  cancel: element<HTMLButtonElement>('cancel'),
  phase: element<HTMLElement>('phase'),
  progressValue: element<HTMLElement>('progress-value'),
  progressBar: element<HTMLElement>('progress-bar'),
  progressDetail: element<HTMLElement>('progress-detail'),
  providerBadge: element<HTMLElement>('provider-badge'),
  timeline: element<HTMLOListElement>('timeline'),
  result: element<HTMLElement>('result'),
  preview: element<HTMLButtonElement>('preview'),
  download: element<HTMLButtonElement>('download'),
  previewRegion: element<HTMLElement>('preview-region'),
  previewContent: element<HTMLElement>('preview-content'),
  closePreview: element<HTMLButtonElement>('close-preview'),
  message: element<HTMLElement>('message'),
}

let selectedFile: File | undefined
let activeController: AbortController | undefined
let completedFile: FileDescriptor | undefined

elements.fileInput.addEventListener('change', () => selectFile(elements.fileInput.files?.[0]))
elements.smallSample.addEventListener('click', () => selectFile(createSampleFile(1)))
elements.largeSample.addEventListener('click', () => selectFile(createSampleFile(150)))
elements.upload.addEventListener('click', () => void uploadSelectedFile())
elements.cancel.addEventListener('click', () => activeController?.abort())
elements.preview.addEventListener('click', () => void previewCompletedFile())
elements.download.addEventListener('click', () => void downloadCompletedFile())
elements.closePreview.addEventListener('click', closePreview)

function selectFile(file: File | undefined): void {
  if (!file || activeController) return
  selectedFile = file
  completedFile = undefined
  closePreview()
  client.clearCache()
  elements.upload.disabled = false
  elements.preview.disabled = true
  elements.download.disabled = true
  elements.providerBadge.textContent = '等待会话'
  elements.result.innerHTML = resultRows()
  resetTimeline()
  setProgress(0, '准备就绪', `${file.name} 已准备，可开始上传。`)
  elements.fileSummary.classList.remove('muted')
  elements.fileSummary.innerHTML = [
    summaryRow('文件', file.name),
    summaryRow('大小', formatBytes(file.size)),
    summaryRow('类型', file.type || 'application/octet-stream'),
  ].join('')
  setMessage('文件只保存在浏览器内存或本机选择结果中；上传开始后字节将直接进入 OSS。', 'neutral')
}

async function uploadSelectedFile(): Promise<void> {
  const file = selectedFile
  if (!file || activeController) return
  activeController = new AbortController()
  setBusy(true)
  completedFile = undefined
  closePreview()
  resetTimeline()
  setStage('session', 'active', '正在向 Evidence 创建受限上传会话')
  setProgress(0, '创建会话', '控制面请求不承载文件字节。')
  setMessage('正在建立上传会话……', 'neutral')

  try {
    const result = await client.uploadFile({
      file,
      signal: activeController.signal,
      clientContext: {
        ownerRef: elements.ownerRef.value.trim() || 'oss-browser-acceptance',
        purpose: elements.purpose.value.trim() || 'external-validation',
      },
      onProgress(progress) {
        setStage('session', 'done', 'Evidence 已签发单 Object Key 的临时 STS')
        setStage('upload', 'active', `浏览器直传 OSS：${progress.percent}%`)
        setProgress(
          progress.percent,
          'OSS multipart 直传',
          `${formatBytes(progress.loadedBytes)} / ${formatBytes(progress.totalBytes)}`,
        )
      },
    })
    acceptUploadResult(result)
  } catch (error) {
    const detail = describeError(error)
    const cancelled = detail.code === 'aborted'
    setStage('upload', 'error', cancelled ? '用户已取消，已请求中止 multipart' : '上传失败')
    setProgress(0, cancelled ? '已取消' : '验收失败', detail.message)
    setMessage(`${detail.code}: ${detail.message}`, 'error')
  } finally {
    activeController = undefined
    setBusy(false)
  }
}

function acceptUploadResult(result: UploadFileResult): void {
  const authorization = result.session.authorization
  const version = result.version
  completedFile = {
    fileId: version.fileId,
    version: version.version,
    fileName: version.fileName,
    sizeBytes: version.sizeBytes,
    mediaType: version.mediaType,
  }
  elements.providerBadge.textContent = authorization.provider
  setStage('session', 'done', 'Evidence 已签发单 Object Key 的临时 STS')
  setStage('upload', 'done', `浏览器已直传 ${formatBytes(version.sizeBytes)}`)
  setStage('complete', 'done', 'Evidence 已重读 OSS 对象并形成不可变 Version')
  setStage('access', 'active', '可以申请短期预览或下载授权')
  setProgress(100, '上传与确认完成', `服务端确认 SHA-256：${abbreviateHash(version.sha256)}`)
  elements.result.innerHTML = resultRows({
    fileId: version.fileId,
    version: String(version.version),
    sha256: version.sha256,
    mediaType: version.mediaType,
    objectKey: authorization.objectKey,
  })
  elements.preview.disabled = client.getPreviewPolicy(completedFile).mode === 'download'
  elements.download.disabled = false
  setMessage('真实 OSS 上传和服务端完整性确认已通过；页面未展示任何 STS Secret 或长期 AccessKey。', 'success')
}

async function previewCompletedFile(): Promise<void> {
  const file = completedFile
  if (!file) return
  elements.preview.disabled = true
  setMessage('正在申请短期 inline 访问授权……', 'neutral')
  try {
    const policy = client.getPreviewPolicy(file)
    if (policy.mode === 'download') throw new Error('当前文件类型或大小只允许下载。')
    const url = await client.resolvePreviewUrl(file, policy.mode)
    elements.previewContent.replaceChildren(createPreviewElement(file, url))
    elements.previewRegion.hidden = false
    setStage('access', 'done', `短期 ${policy.mode} 预览授权已验证`)
    setMessage('预览已通过 Evidence 授权；短期签名 URL 不写入日志或持久存储。', 'success')
  } catch (error) {
    const detail = describeError(error)
    setStage('access', 'error', '预览授权或读取失败')
    setMessage(`${detail.code}: ${detail.message}`, 'error')
  } finally {
    elements.preview.disabled = client.getPreviewPolicy(file).mode === 'download'
  }
}

async function downloadCompletedFile(): Promise<void> {
  const file = completedFile
  if (!file) return
  elements.download.disabled = true
  setMessage('正在申请短期 attachment 访问授权……', 'neutral')
  try {
    await client.downloadFile(file)
    setStage('access', 'done', '短期下载授权已签发并触发浏览器下载')
    setMessage('下载已触发；请用本机文件校验实际字节。', 'success')
  } catch (error) {
    const detail = describeError(error)
    setStage('access', 'error', '下载授权失败')
    setMessage(`${detail.code}: ${detail.message}`, 'error')
  } finally {
    elements.download.disabled = false
  }
}

function setBusy(busy: boolean): void {
  elements.upload.disabled = busy || !selectedFile
  elements.cancel.disabled = !busy
  elements.fileInput.disabled = busy
  elements.smallSample.disabled = busy
  elements.largeSample.disabled = busy
  elements.ownerRef.disabled = busy
  elements.purpose.disabled = busy
}

function setProgress(percent: number, phase: string, detail: string): void {
  const bounded = Math.max(0, Math.min(100, percent))
  elements.phase.textContent = phase
  elements.progressValue.textContent = `${Math.round(bounded)}%`
  elements.progressBar.style.width = `${bounded}%`
  elements.progressDetail.textContent = detail
}

function setStage(stage: string, state: 'active' | 'done' | 'error', detail: string): void {
  const item = elements.timeline.querySelector<HTMLElement>(`[data-stage="${stage}"]`)
  if (!item) return
  item.dataset.state = state
  const copy = item.querySelector('p')
  if (copy) copy.textContent = detail
}

function resetTimeline(): void {
  const defaults: Record<string, string> = {
    session: '尚未创建',
    upload: '尚未开始',
    complete: '尚未完成',
    access: '等待可用版本',
  }
  for (const item of elements.timeline.querySelectorAll<HTMLElement>('[data-stage]')) {
    delete item.dataset.state
    const stage = item.dataset.stage
    const copy = item.querySelector('p')
    if (copy && stage) copy.textContent = defaults[stage] ?? '等待'
  }
}

function closePreview(): void {
  elements.previewRegion.hidden = true
  elements.previewContent.replaceChildren()
}

function setMessage(copy: string, tone: 'neutral' | 'success' | 'error'): void {
  elements.message.textContent = copy
  elements.message.dataset.tone = tone
}

function createSampleFile(mebibytes: number): File {
  const chunk = new Uint8Array(1024 * 1024)
  const parts = Array.from({ length: mebibytes }, () => chunk)
  return new File(parts, `oss-validation-${mebibytes}MiB.bin`, {
    type: 'application/octet-stream',
    lastModified: Date.now(),
  })
}

function createPreviewElement(file: FileDescriptor, url: string): HTMLElement {
  if (file.mediaType?.startsWith('image/')) {
    const image = document.createElement('img')
    image.src = url
    image.alt = file.fileName
    return image
  }
  const frame = document.createElement('iframe')
  frame.src = url
  frame.title = file.fileName
  return frame
}

function resultRows(values: {
  fileId?: string
  version?: string
  sha256?: string
  mediaType?: string
  objectKey?: string
} = {}): string {
  return [
    summaryRow('File ID', values.fileId ?? '—'),
    summaryRow('Version', values.version ?? '—'),
    summaryRow('SHA-256', values.sha256 ?? '—'),
    summaryRow('服务端类型', values.mediaType ?? '—'),
    summaryRow('Object Key', values.objectKey ?? '—'),
  ].join('')
}

function summaryRow(label: string, value: string): string {
  return `<div><dt>${escapeHtml(label)}</dt><dd>${escapeHtml(value)}</dd></div>`
}

function describeError(error: unknown): { code: string; message: string } {
  if (error instanceof FileTransferError) return { code: error.code, message: error.message }
  if (error instanceof Error) return { code: error.name || 'error', message: error.message }
  return { code: 'unknown_error', message: '发生未知错误。' }
}

function abbreviateHash(value: string): string {
  return value.length > 20 ? `${value.slice(0, 12)}…${value.slice(-8)}` : value
}

function formatBytes(bytes: number): string {
  if (bytes < 1024) return `${bytes} B`
  const units = ['KiB', 'MiB', 'GiB']
  let value = bytes / 1024
  let index = 0
  while (value >= 1024 && index < units.length - 1) {
    value /= 1024
    index += 1
  }
  return `${value.toFixed(value >= 10 ? 1 : 2)} ${units[index]}`
}

function escapeHtml(value: string): string {
  const node = document.createElement('span')
  node.textContent = value
  return node.innerHTML
}

function element<T extends HTMLElement>(id: string): T {
  const value = document.getElementById(id)
  if (!value) throw new Error(`Missing required element #${id}.`)
  return value as T
}

