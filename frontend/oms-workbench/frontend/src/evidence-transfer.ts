import {
  FileTransferClient,
  createAliOssMultipartUploader,
  createDirectUploaderRouter,
  createFetchFileTransferApi,
  createLocalHttpUploader,
} from '../../../../packages/file-transfer-core/src/index'

// Current OMS owns this narrow browser adapter. It intentionally excludes the
// retired reference OA React tree while preserving the audited Evidence API,
// local HTTP upload and bounded Aliyun OSS STS multipart implementation.
export function createEvidenceTransfer(
  evidenceFetch: (input: RequestInfo | URL, options?: RequestInit) => Promise<Response>,
) {
  return new FileTransferClient({
    api: createFetchFileTransferApi({baseUrl: '', fetcher: evidenceFetch}),
    uploader: createDirectUploaderRouter({
      aliyunOss: createAliOssMultipartUploader(),
      localHttp: createLocalHttpUploader(),
    }),
    getCacheScope: () => undefined,
  })
}
