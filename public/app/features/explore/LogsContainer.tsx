import React, { PureComponent } from 'react';
import { isEqual } from 'lodash';
import { hot } from 'react-hot-loader';
import { connect, ConnectedProps } from 'react-redux';
import { Collapse } from '@grafana/ui';
import LRU from 'lru-cache';

import {
  Field,
  LogLevel,
  LogRowModel,
  LogsDedupStrategy,
  RawTimeRange,
  DataQuery,
  AbsoluteTimeRange,
  LogsModel,
} from '@grafana/data';

import { ExploreId, ExploreItemState } from 'app/types/explore';
import { StoreState } from 'app/types';

import { splitOpen } from './state/main';
import { updateTimeRange } from './state/time';
import { toggleLogLevelAction, changeDedupStrategy } from './state/explorePane';
import { deduplicatedRowsSelector } from './state/selectors';
import { getTimeZone } from '../profile/state/selectors';
import { LiveLogsWithTheme } from './LiveLogs';
import { Logs } from './Logs';
import { LogsCrossFadeTransition } from './utils/LogsCrossFadeTransition';
import { LiveTailControls } from './useLiveTailControls';
import { getFieldLinksForExplore } from './utils/links';
import { queries } from '@testing-library/react';

interface LogsContainerProps {
  exploreId: ExploreId;
  scanRange?: RawTimeRange;
  width: number;
  syncedTimes: boolean;
  onClickFilterLabel?: (key: string, value: string) => void;
  onClickFilterOutLabel?: (key: string, value: string) => void;
  onStartScanning: () => void;
  onStopScanning: () => void;
}
type State = {
  toShowResult: LogsModel | null;
  toShowAbsoluteRange: AbsoluteTimeRange;
};

export class LogsContainer extends PureComponent<PropsFromRedux & LogsContainerProps, State> {
  private logRowsCache = new LRU<string, { logResult: LogsModel; absRange: AbsoluteTimeRange }>(5);
  constructor(props: PropsFromRedux & LogsContainerProps) {
    super(props);

    this.state = {
      toShowResult: props.logResult,
      toShowAbsoluteRange: props.absoluteRange,
    };
  }

  componentDidMount() {
    if (this.props.logResult && this.props.absoluteRange && this.props.queries) {
      const params = {
        from: this.props.absoluteRange.from,
        to: this.props.absoluteRange.from,
        queries: this.props.queries.map((q) => q.key),
      };
      const cacheKey = Object.entries(params)
        .map(([k, v]) => `${encodeURIComponent(k)}=${encodeURIComponent(v.toString())}`)
        .join('&');
      this.logRowsCache.set(cacheKey, { logResult: this.props.logResult, absRange: this.props.absoluteRange });
    }
  }

  componentDidUpdate(prevProps: PropsFromRedux & LogsContainerProps) {
    const { queries, logResult, absoluteRange } = this.props;
    if (logResult && isEqual(logResult, prevProps.logResult)) {
      const params = {
        from: absoluteRange.from,
        to: absoluteRange.to,
        queries: queries.map((q) => q.key),
      };
      const cacheKey = Object.entries(params)
        .map(([k, v]) => `${encodeURIComponent(k)}=${encodeURIComponent(v.toString())}`)
        .join('&');
      this.logRowsCache.set(cacheKey, { logResult, absRange: absoluteRange });
    }
  }

  onChangeTime = (absoluteRange: AbsoluteTimeRange, queries?: DataQuery[]) => {
    const { exploreId, updateTimeRange } = this.props;
    if (!queries) {
      updateTimeRange({ exploreId, absoluteRange });
    } else {
      const cacheParams = {
        from: absoluteRange.from,
        to: absoluteRange.to,
        queries: queries.map((q) => q.key),
      };

      const cacheKey = Object.entries(cacheParams)
        .map(([k, v]) => `${encodeURIComponent(k)}=${encodeURIComponent(v.toString())}`)
        .join('&');

      console.log(cacheKey);
      let { logResult, absRange } = this.logRowsCache.get(cacheKey) || {};
      if (!logResult || !absRange) {
        console.log('updating time');
        updateTimeRange({ exploreId, absoluteRange });
      } else {
        console.log('seting cached results: ');
        console.log(logResult);
        this.setState({ toShowResult: logResult });
        this.setState({ toShowAbsoluteRange: absRange });
      }
    }
  };

  handleDedupStrategyChange = (dedupStrategy: LogsDedupStrategy) => {
    this.props.changeDedupStrategy(this.props.exploreId, dedupStrategy);
  };

  handleToggleLogLevel = (hiddenLogLevels: LogLevel[]) => {
    const { exploreId } = this.props;
    this.props.toggleLogLevelAction({
      exploreId,
      hiddenLogLevels,
    });
  };

  getLogRowContext = async (row: LogRowModel, options?: any): Promise<any> => {
    const { datasourceInstance } = this.props;

    if (datasourceInstance?.getLogRowContext) {
      return datasourceInstance.getLogRowContext(row, options);
    }

    return [];
  };

  showContextToggle = (row?: LogRowModel): boolean => {
    const { datasourceInstance } = this.props;

    if (datasourceInstance?.showContextToggle) {
      return datasourceInstance.showContextToggle(row);
    }

    return false;
  };

  getFieldLinks = (field: Field, rowIndex: number) => {
    const { splitOpen: splitOpenFn, range } = this.props;
    return getFieldLinksForExplore({ field, rowIndex, splitOpenFn, range });
  };

  render() {
    const {
      loading,
      logsHighlighterExpressions,
      dedupedRows,
      onClickFilterLabel,
      onClickFilterOutLabel,
      onStartScanning,
      onStopScanning,
      absoluteRange,
      timeZone,
      scanning,
      range,
      width,
      isLive,
      exploreId,
      queries,
    } = this.props;

    const { toShowResult, toShowAbsoluteRange } = this.state;

    const { rows: logRows, meta: logsMeta, series: logsSeries, visibleRange } = toShowResult || {};

    if (!logRows) {
      return null;
    }

    return (
      <>
        <LogsCrossFadeTransition visible={isLive}>
          <Collapse label="Logs" loading={false} isOpen>
            <LiveTailControls exploreId={exploreId}>
              {(controls) => (
                <LiveLogsWithTheme
                  logRows={logRows}
                  timeZone={timeZone}
                  stopLive={controls.stop}
                  isPaused={this.props.isPaused}
                  onPause={controls.pause}
                  onResume={controls.resume}
                />
              )}
            </LiveTailControls>
          </Collapse>
        </LogsCrossFadeTransition>
        <LogsCrossFadeTransition visible={!isLive}>
          <Collapse label="Logs" loading={loading} isOpen>
            <Logs
              dedupStrategy={this.props.dedupStrategy || LogsDedupStrategy.none}
              logRows={logRows}
              logsMeta={logsMeta}
              logsSeries={logsSeries}
              dedupedRows={dedupedRows}
              highlighterExpressions={logsHighlighterExpressions}
              loading={loading}
              onChangeTime={this.onChangeTime}
              onClickFilterLabel={onClickFilterLabel}
              onClickFilterOutLabel={onClickFilterOutLabel}
              onStartScanning={onStartScanning}
              onStopScanning={onStopScanning}
              onDedupStrategyChange={this.handleDedupStrategyChange}
              onToggleLogLevel={this.handleToggleLogLevel}
              absoluteRange={toShowAbsoluteRange}
              visibleRange={visibleRange}
              timeZone={timeZone}
              scanning={scanning}
              scanRange={range.raw}
              showContextToggle={this.showContextToggle}
              width={width}
              getRowContext={this.getLogRowContext}
              getFieldLinks={this.getFieldLinks}
              queries={queries}
            />
          </Collapse>
        </LogsCrossFadeTransition>
      </>
    );
  }
}

function mapStateToProps(state: StoreState, { exploreId }: { exploreId: string }) {
  const explore = state.explore;
  // @ts-ignore
  const item: ExploreItemState = explore[exploreId];
  const {
    logsHighlighterExpressions,
    logsResult,
    loading,
    scanning,
    datasourceInstance,
    isLive,
    isPaused,
    range,
    absoluteRange,
    dedupStrategy,
    queries,
  } = item;
  const dedupedRows = deduplicatedRowsSelector(item) || undefined;
  const timeZone = getTimeZone(state.user);

  return {
    loading,
    logsHighlighterExpressions,
    logResult: logsResult,
    scanning,
    timeZone,
    dedupStrategy,
    dedupedRows,
    datasourceInstance,
    isLive,
    isPaused,
    range,
    absoluteRange,
    queries,
  };
}

const mapDispatchToProps = {
  changeDedupStrategy,
  toggleLogLevelAction,
  updateTimeRange,
  splitOpen,
};

const connector = connect(mapStateToProps, mapDispatchToProps);
type PropsFromRedux = ConnectedProps<typeof connector>;

export default hot(module)(connector(LogsContainer));
