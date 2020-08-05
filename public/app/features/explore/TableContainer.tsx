import React, { PureComponent } from 'react';
import { hot } from 'react-hot-loader';
import { connect } from 'react-redux';
import { DataFrame, TimeRange, ValueLinkConfig } from '@grafana/data';
import { Collapse, Table } from '@grafana/ui';
import { ExploreId, ExploreItemState } from 'app/types/explore';
import { StoreState } from 'app/types';
import { splitOpen, toggleTable } from './state/actions';
import { config } from 'app/core/config';
import { PANEL_BORDER } from 'app/core/constants';
import { MetaInfoText } from './MetaInfoText';
import { FilterItem } from '@grafana/ui/src/components/Table/types';
import { getFieldLinksForExplore } from './utils/links';

interface TableContainerProps {
  exploreId: ExploreId;
  loading: boolean;
  width: number;
  onCellFilterAdded?: (filter: FilterItem) => void;
  showingTable: boolean;
  tableResults?: DataFrame[];
  toggleTable: typeof toggleTable;
  splitOpen: typeof splitOpen;
  range: TimeRange;
}

export class TableContainer extends PureComponent<TableContainerProps> {
  onClickTableButton = () => {
    this.props.toggleTable(this.props.exploreId, this.props.showingTable);
  };

  getTableHeight() {
    const { tableResults } = this.props;

    if (!tableResults || tableResults.length === 0) {
      return 200;
    }

    // tries to estimate table height
    return Math.max(Math.min(600, tableResults.length * 35) + 35);
  }

  render() {
    const { loading, onCellFilterAdded, showingTable, tableResults, width, splitOpen, range } = this.props;

    const height = this.getTableHeight();
    const tableWidth = width - config.theme.panelPadding * 2 - PANEL_BORDER;

    if (tableResults?.length) {
      // Bit of code smell here. We need to add links here to the frame modifying the frame on every render.
      // Should work fine in essence but still not the ideal way to pass props. In logs container we do this
      // differently and sidestep this getLinks API on a dataframe
      tableResults.forEach(tableResult => {
        for (const field of tableResult.fields) {
          field.getLinks = (config: ValueLinkConfig) => {
            return getFieldLinksForExplore(field, config.valueRowIndex!, splitOpen, range);
          };
        }
      });
    }

    if (!tableResults?.length) {
      return <MetaInfoText metaItems={[{ value: '0 series returned' }]} />;
    }

    return (
      <>
        {tableResults.map((tableResult, index) => {
          if (!tableResult.length) {
            return null;
          }
          return (
            <Collapse
              label="Table"
              key={tableResult.name || index}
              loading={loading}
              collapsible
              isOpen={showingTable}
              onToggle={this.onClickTableButton}
            >
              <Table data={tableResult} width={tableWidth} height={height} onCellFilterAdded={onCellFilterAdded} />
            </Collapse>
          );
        })}
      </>
    );
  }
}

function mapStateToProps(state: StoreState, { exploreId }: { exploreId: string }) {
  const explore = state.explore;
  // @ts-ignore
  const item: ExploreItemState = explore[exploreId];
  const { loading: loadingInState, showingTable, tableResults, range } = item;
  const loading = tableResults && tableResults.length > 0 ? false : loadingInState;
  return { loading, showingTable, tableResults, range };
}

const mapDispatchToProps = {
  toggleTable,
  splitOpen,
};

export default hot(module)(connect(mapStateToProps, mapDispatchToProps)(TableContainer));
