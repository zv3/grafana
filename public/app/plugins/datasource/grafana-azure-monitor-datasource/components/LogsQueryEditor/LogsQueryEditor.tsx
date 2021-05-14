import React, { useCallback, useState } from 'react';
import { AzureMonitorErrorish, AzureMonitorOption, AzureMonitorQuery } from '../../types';
import Datasource from '../../datasource';
import { Modal } from '@grafana/ui';

import QueryField from './QueryField';
import FormatAsField from './FormatAsField';
import ResourcePicker from '../ResourcePicker';
import { Row, RowGroup } from '../ResourcePicker/types';

interface LogsQueryEditorProps {
  query: AzureMonitorQuery;
  datasource: Datasource;
  subscriptionId: string;
  onChange: (newQuery: AzureMonitorQuery) => void;
  variableOptionGroup: { label: string; options: AzureMonitorOption[] };
  setError: (source: string, error: AzureMonitorErrorish | undefined) => void;
}

const LogsQueryEditor: React.FC<LogsQueryEditorProps> = ({
  query,
  datasource,
  subscriptionId,
  variableOptionGroup,
  onChange,
  setError,
}) => {
  // TODO: handle saved queries and translating that scope into a selected resource.
  // TODO: how should we handle opening empty folders?
  const [selectedResource, setSelectedResource] = useState<RowGroup>({});
  const handleSelectResource = useCallback(
    (row: Row, isSelected: boolean) => {
      if (isSelected) {
        setSelectedResource({ [row.id]: row });
        onChange({
          ...query,
          subscription: row.subscriptionId,
          azureLogAnalytics: {
            ...query.azureLogAnalytics,
            workspace: row.name, // TODO: not sure what to put here? Josh mentioned something about a resource uri?
          },
        });
      } else {
        setSelectedResource({});
      }
    },
    [onChange, setSelectedResource, query]
  );

  const [isResourcePickerOpen, setIsResourcePickerOpen] = useState(false);

  return (
    <div data-testid="azure-monitor-logs-query-editor">
      <Modal title="Select a resource" isOpen={isResourcePickerOpen} onDismiss={() => setIsResourcePickerOpen(false)}>
        <ResourcePicker
          resourcePickerData={datasource.resourcePickerData}
          selectedResource={selectedResource}
          handleSelectResource={handleSelectResource}
        />
      </Modal>

      {/* TODO use a real component here, some kind of breadcrumb like button */}
      <button onClick={() => setIsResourcePickerOpen(true)}>Change Scope</button>

      <QueryField
        query={query}
        datasource={datasource}
        subscriptionId={subscriptionId}
        variableOptionGroup={variableOptionGroup}
        onQueryChange={onChange}
        setError={setError}
      />

      <FormatAsField
        query={query}
        datasource={datasource}
        subscriptionId={subscriptionId}
        variableOptionGroup={variableOptionGroup}
        onQueryChange={onChange}
        setError={setError}
      />
    </div>
  );
};

export default LogsQueryEditor;
