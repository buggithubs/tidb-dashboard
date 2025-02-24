import React, { useCallback, useState } from 'react'
import { Select, Tooltip } from 'antd'
import { useTranslation } from 'react-i18next'

import client, { InfoTableSchema } from '@lib/client'
import type { ApiFormWidget } from './index'
import { useLimitSelection } from './useLimitSelection'

const filterOptionByNameAndID: any = (
  inputValue: string,
  // children means Select.Option children nodes
  option: { children: string }
) => {
  return option.children.includes(inputValue)
}

export const TableIDWidget: ApiFormWidget = ({ value, onChange }) => {
  const { t } = useTranslation()
  const tips = t(`debug_api.widgets.table_id`)

  const [loading, setLoading] = useState(false)
  const [options, setOptions] = useState<InfoTableSchema[]>([])
  const onFocus = useCallback(async () => {
    if (options.length) {
      return
    }

    setLoading(true)
    try {
      const rst = await client.getInstance().infoListTables()
      setOptions(rst.data)
    } finally {
      setLoading(false)
    }
  }, [setLoading, setOptions, options])

  const memoOnChange = useCallback(
    (tags: string[]) => onChange?.(tags[0]),
    [onChange]
  )
  const { selectRef, onSelectChange } = useLimitSelection(1, memoOnChange)

  return (
    <Tooltip trigger={['focus']} title={tips} placement="topLeft">
      <Select
        ref={selectRef}
        mode="tags"
        dropdownStyle={{ visibility: loading ? 'hidden' : 'visible' }}
        loading={loading}
        placeholder={tips}
        value={value ? [value] : []}
        onFocus={onFocus}
        onChange={onSelectChange}
        filterOption={filterOptionByNameAndID}
      >
        {options.map((option) => (
          <Select.Option key={option.table_id!} value={option.table_id!}>
            {`(${option.table_name})${option.table_id}`}
          </Select.Option>
        ))}
      </Select>
    </Tooltip>
  )
}
