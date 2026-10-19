import { Select } from 'antd'
import type { SelectProps } from 'antd'
import type { EmploymentStatus, User } from '@/types'
import { getPersonnelName, isPersonnelResigned } from './PersonnelName'
import './PersonnelSelect.css'

export interface PersonnelSelectOption {
  value: string | number
  user?: Pick<User, 'id' | 'username' | 'real_name'> &
    Partial<Pick<User, 'phone' | 'department' | 'employment_status'>>
  label?: string
  description?: string
  disabled?: boolean
}

interface PersonnelSelectProps
  extends Omit<
    SelectProps<any>,
    'options' | 'optionRender' | 'labelRender' | 'optionFilterProp'
  > {
  users?: User[]
  personnelOptions?: PersonnelSelectOption[]
}

const isResigned = (user?: { employment_status?: EmploymentStatus }) =>
  isPersonnelResigned(user)

const createUserOption = (user: User): PersonnelSelectOption => ({
  value: user.id,
  user,
})

export const PersonnelSelect = ({
  users = [],
  personnelOptions,
  className,
  popupClassName,
  listHeight = 368,
  showSearch = true,
  allowClear = true,
  ...selectProps
}: PersonnelSelectProps) => {
  const sourceOptions = personnelOptions ?? users.map(createUserOption)
  const optionMap = new Map(
    sourceOptions.map((option) => [String(option.value), option])
  )

  const options = sourceOptions.map((option) => {
    const user = option.user
    const name = option.label || (user ? getPersonnelName(user) : String(option.value))
    const resignedLabel = isResigned(user) ? ' 已离职' : ''
    const description =
      option.description ||
      [user?.department, user?.username && user.username !== name ? user.username : null]
        .filter(Boolean)
        .join(' · ')

    return {
      value: option.value,
      label: `${name}${resignedLabel}`,
      searchText: [name, user?.username, user?.phone, user?.department, resignedLabel]
        .filter(Boolean)
        .join(' '),
      description,
      disabled: option.disabled,
      personnelOption: option,
    }
  })

  const renderSelectedLabel = (value: string | number, fallback?: React.ReactNode) => {
    const option = optionMap.get(String(value))
    if (!option) return fallback
    const user = option.user
    const name = option.label || (user ? getPersonnelName(user) : String(value))

    return (
      <span
        className={[
          'personnel-select__selected',
          isResigned(user) ? 'personnel-select__selected--resigned' : '',
        ]
          .filter(Boolean)
          .join(' ')}
      >
        {name}
        {isResigned(user) && <em>（已离职）</em>}
      </span>
    )
  }

  return (
    <Select
      {...selectProps}
      className={['personnel-select', className].filter(Boolean).join(' ')}
      popupClassName={[
        'personnel-select__popup',
        popupClassName,
      ]
        .filter(Boolean)
        .join(' ')}
      showSearch={showSearch}
      allowClear={allowClear}
      listHeight={listHeight}
      options={options}
      filterOption={(input, option) =>
        String(option?.searchText || '')
          .toLocaleLowerCase()
          .includes(input.trim().toLocaleLowerCase())
      }
      optionRender={(option) => {
        const data = option.data as (typeof options)[number]
        const source = data.personnelOption
        const user = source.user
        const name = source.label || (user ? getPersonnelName(user) : String(source.value))
        const resigned = isResigned(user)

        return (
          <div
            className={[
              'personnel-select__option',
              resigned ? 'personnel-select__option--resigned' : '',
            ]
              .filter(Boolean)
              .join(' ')}
          >
            <div className="personnel-select__option-main">
              <strong>{name}</strong>
              {resigned && <span>已离职</span>}
            </div>
            {data.description && (
              <small>{data.description}</small>
            )}
          </div>
        )
      }}
      labelRender={({ value, label }) => renderSelectedLabel(value, label)}
    />
  )
}

export default PersonnelSelect
