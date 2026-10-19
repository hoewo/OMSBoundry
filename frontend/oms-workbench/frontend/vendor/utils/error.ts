const TRANSPORT_ERROR_PATTERN =
  /Request failed with status code|Network Error|timeout|network timeout/i

const KNOWN_ERROR_MAPPINGS: Array<{ pattern: RegExp; message: string }> = [
  {
    pattern: /contract number already exists|合同编号已存在/i,
    message: '合同编号已存在，请使用其他编号',
  },
  {
    pattern: /amendment number already exists|补充协议编号已存在/i,
    message: '补充协议编号已存在，请使用其他编号',
  },
  {
    pattern:
      /project number already exists|项目编号已存在|idx_projects_project_number|project_number.*SQLSTATE 23505|SQLSTATE 23505.*project_number/i,
    message: '项目编号已存在，请使用其他编号',
  },
  {
    pattern: /client name already exists|甲方名称已存在/i,
    message: '甲方名称已存在，请使用已存在的甲方',
  },
  {
    pattern: /email already exists|邮箱已被使用/i,
    message: '邮箱已被使用',
  },
  {
    pattern: /username already exists|phone already exists|手机号已被使用/i,
    message: '手机号已被使用',
  },
]

const pickBackendMessage = (data: any): string | undefined => {
  if (!data) {
    return undefined
  }

  if (typeof data === 'string') {
    return data
  }

  if (typeof data.error === 'string' && data.error.trim()) {
    return data.error.trim()
  }

  if (typeof data.message === 'string' && data.message.trim()) {
    return data.message.trim()
  }

  if (Array.isArray(data.errors) && data.errors.length > 0) {
    const firstError = data.errors.find(
      (item: unknown) => typeof item === 'string'
    )
    if (typeof firstError === 'string' && firstError.trim()) {
      return firstError.trim()
    }
  }

  return undefined
}

const normalizeKnownMessage = (message: string): string => {
  const trimmed = message.trim()
  if (!trimmed) {
    return ''
  }

  const mapped = KNOWN_ERROR_MAPPINGS.find((item) => item.pattern.test(trimmed))
  if (mapped) {
    return mapped.message
  }

  const failedPrefixMatch = trimmed.match(/^Failed to [^:]+:\s*(.+)$/i)
  if (failedPrefixMatch?.[1]) {
    const nested = failedPrefixMatch[1].trim()
    const nestedMapped = KNOWN_ERROR_MAPPINGS.find((item) =>
      item.pattern.test(nested)
    )
    return nestedMapped?.message || nested
  }

  return trimmed
}

const getRawErrorCandidates = (error: any): string[] => {
  const responseData = error?.response?.data

  return [
    pickBackendMessage(responseData),
    typeof error?.message === 'string' ? error.message : undefined,
  ].filter((item): item is string => !!item && item.trim().length > 0)
}

/**
 * 从 Axios/业务错误对象中提取用户友好的错误信息
 * 优先返回后端返回的具体错误信息，如果没有则返回默认消息
 */
export function getErrorMessage(
  error: any,
  defaultMessage: string = '操作失败'
): string {
  if (typeof error === 'string') {
    return normalizeKnownMessage(error) || defaultMessage
  }

  if (error?.response?.status === 403) {
    return '权限已变更或当前账号无权操作，请刷新页面后重试'
  }

  for (const candidate of getRawErrorCandidates(error)) {
    if (TRANSPORT_ERROR_PATTERN.test(candidate)) {
      continue
    }

    const normalized = normalizeKnownMessage(candidate)
    if (normalized) {
      return normalized
    }
  }

  return defaultMessage
}

export function errorIncludes(
  error: any,
  ...patterns: Array<string | RegExp>
): boolean {
  const haystack = getRawErrorCandidates(error).join(' | ').toLowerCase()

  return patterns.some((pattern) =>
    typeof pattern === 'string'
      ? haystack.includes(pattern.toLowerCase())
      : pattern.test(haystack)
  )
}
