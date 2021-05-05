import React, { useCallback, useState } from 'react';
import { Input } from '@grafana/ui';

import { Field } from '../Field';
import { AzureQueryEditorFieldProps } from '../../types';

const ResourceField: React.FC<AzureQueryEditorFieldProps> = ({ onQueryChange, query }) => {
  const [value, setValue] = useState<string>(query.azureLogAnalytics.resource ?? '');

  // As calling onQueryChange initiates a the datasource refresh, we only want to call it once
  // the field loses focus
  const handleChange = useCallback((ev: React.FormEvent) => {
    if (ev.target instanceof HTMLInputElement) {
      setValue(ev.target.value);
    }
  }, []);

  const handleBlur = useCallback(() => {
    onQueryChange({
      ...query,
      azureLogAnalytics: {
        ...query.azureLogAnalytics,
        resource: value,
      },
    });
  }, [onQueryChange, query, value]);

  return (
    <Field label="Resource">
      <Input
        id="azure-monitor-logs-resource-field"
        placeholder="Fully qualified resource URI"
        value={value}
        onChange={handleChange}
        onBlur={handleBlur}
        width={100}
      />
    </Field>
  );
};

export default ResourceField;
